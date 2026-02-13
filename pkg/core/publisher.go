package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmamaqp "github.com/ThreeDotsLabs/watermill-amqp/pkg/amqp"
	wmhttp "github.com/ThreeDotsLabs/watermill-http/v2/pkg/http"
	wmkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	stan "github.com/nats-io/stan.go"
	"google.golang.org/protobuf/proto"

	cloudv1 "githook/pkg/gen/cloud/v1"
)

// Publisher defines the interface for publishing events.
type Publisher interface {
	// Publish sends an event to a specific topic.
	Publish(ctx context.Context, topic string, event Event) error
	// PublishForDrivers sends an event to a specific topic for a given set of drivers.
	PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error
	// Close gracefully closes the publisher and its underlying connections.
	Close() error
}

// watermillPublisher is a wrapper around a Watermill message.Publisher.
type watermillPublisher struct {
	publisher message.Publisher
	closeFn   func() error
}

// PublisherFactory is a function that creates a new Watermill publisher.
type PublisherFactory func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error)

var publisherFactories = map[string]PublisherFactory{
	"gochannel": buildGoChannelPublisher,
}

// RegisterPublisherDriver registers a new publisher driver.
func RegisterPublisherDriver(name string, factory PublisherFactory) {
	if name == "" || factory == nil {
		return
	}
	publisherFactories[strings.ToLower(name)] = factory
}

// NewPublisher creates a new publisher based on the provided configuration.
// It can create multiple publishers if multiple drivers are configured.
func NewPublisher(cfg WatermillConfig) (Publisher, error) {
	logger := watermill.NewStdLogger(false, false)

	drivers := cfg.Drivers
	if len(drivers) == 0 && cfg.Driver != "" {
		drivers = []string{cfg.Driver}
	}
	if len(drivers) == 0 {
		drivers = []string{"gochannel"}
	}

	pubs := make(map[string]Publisher, len(drivers))
	builtDrivers := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		pub, err := retryPublisherBuild(func() (Publisher, error) {
			return newSinglePublisher(cfg, driver)
		})
		if err != nil {
			logger.Error("publisher init failed, skipping driver", err, watermill.LogFields{
				"driver": driver,
			})
			continue
		}
		key := strings.ToLower(driver)
		pubs[key] = pub
		builtDrivers = append(builtDrivers, key)
	}
	if len(pubs) == 0 {
		return nil, errors.New("no publishers available")
	}
	return &publisherMux{
		publishers:     pubs,
		defaultDrivers: builtDrivers,
		retryAttempts:  cfg.PublishRetry.Attempts,
		retryDelay:     time.Duration(cfg.PublishRetry.DelayMS) * time.Millisecond,
		dlqDriver:      strings.ToLower(strings.TrimSpace(cfg.DLQDriver)),
	}, nil
}

func newSinglePublisher(cfg WatermillConfig, driver string) (Publisher, error) {
	logger := watermill.NewStdLogger(false, false)

	switch strings.ToLower(driver) {
	case "http":
		targetMode := strings.ToLower(cfg.HTTP.Mode)
		if targetMode != "topic_url" && targetMode != "base_url" {
			return nil, fmt.Errorf("unsupported http mode: %s", cfg.HTTP.Mode)
		}
		if targetMode == "base_url" && cfg.HTTP.BaseURL == "" {
			return nil, fmt.Errorf("http base_url is required for base_url mode")
		}
		pub, err := wmhttp.NewPublisher(wmhttp.PublisherConfig{
			MarshalMessageFunc: func(topic string, msg *message.Message) (*http.Request, error) {
				target, err := httpTargetURL(cfg.HTTP, topic)
				if err != nil {
					return nil, err
				}
				return wmhttp.DefaultMarshalMessageFunc(target, msg)
			},
		}, logger)
		if err != nil {
			return nil, err
		}
		return &watermillPublisher{publisher: pub}, nil
	case "kafka":
		if len(cfg.Kafka.Brokers) == 0 {
			return nil, fmt.Errorf("kafka brokers are required")
		}
		pub, err := retryPublisher(func() (message.Publisher, error) {
			return wmkafka.NewPublisher(cfg.Kafka.Brokers, wmkafka.DefaultMarshaler{}, nil, logger)
		})
		if err != nil {
			return nil, err
		}
		return &watermillPublisher{publisher: pub}, nil
	case "nats":
		if cfg.NATS.ClusterID == "" || cfg.NATS.ClientID == "" {
			return nil, fmt.Errorf("nats cluster_id and client_id are required")
		}
		natsCfg := wmnats.StreamingPublisherConfig{
			ClusterID: cfg.NATS.ClusterID,
			ClientID:  cfg.NATS.ClientID,
			Marshaler: wmnats.GobMarshaler{},
		}
		if cfg.NATS.URL != "" {
			natsCfg.StanOptions = append(natsCfg.StanOptions, stan.NatsURL(cfg.NATS.URL))
		}
		pub, err := wmnats.NewStreamingPublisher(natsCfg, logger)
		if err != nil {
			return nil, err
		}
		return &watermillPublisher{publisher: pub}, nil
	case "amqp":
		if cfg.AMQP.URL == "" {
			return nil, fmt.Errorf("amqp url is required")
		}
		amqpCfg, err := amqpConfigFromMode(cfg.AMQP.URL, cfg.AMQP.Mode)
		if err != nil {
			return nil, err
		}
		pub, err := wmamaqp.NewPublisher(amqpCfg, logger)
		if err != nil {
			return nil, err
		}
		return &watermillPublisher{publisher: pub}, nil
	case "sql":
		if cfg.SQL.Driver == "" || cfg.SQL.DSN == "" {
			return nil, fmt.Errorf("sql driver and dsn are required")
		}
		schemaAdapter, err := sqlSchemaAdapter(cfg.SQL.Dialect)
		if err != nil {
			return nil, err
		}
		db, err := sql.Open(cfg.SQL.Driver, cfg.SQL.DSN)
		if err != nil {
			return nil, err
		}
		autoInit := cfg.SQL.AutoInitializeSchema || cfg.SQL.InitializeSchema
		pub, err := wmsql.NewPublisher(db, wmsql.PublisherConfig{
			SchemaAdapter:        schemaAdapter,
			AutoInitializeSchema: autoInit,
		}, logger)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		return &watermillPublisher{
			publisher: pub,
			closeFn:   db.Close,
		}, nil
	case "riverqueue":
		pub, err := newRiverQueuePublisher(cfg.RiverQueue)
		if err != nil {
			return nil, err
		}
		return pub, nil
	default:
		if factory, ok := publisherFactories[strings.ToLower(driver)]; ok {
			pub, closeFn, err := factory(cfg, logger)
			if err != nil {
				return nil, err
			}
			return &watermillPublisher{publisher: pub, closeFn: closeFn}, nil
		}
		return nil, fmt.Errorf("unsupported watermill driver: %s", driver)
	}
}

func retryPublisher(build func() (message.Publisher, error)) (message.Publisher, error) {
	const attempts = 10
	const delay = 2 * time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		pub, err := build()
		if err == nil {
			return pub, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	return nil, lastErr
}

func retryPublisherBuild(build func() (Publisher, error)) (Publisher, error) {
	const attempts = 10
	const delay = 2 * time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		pub, err := build()
		if err == nil {
			return pub, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	return nil, lastErr
}

// Publish sends an event to a topic using the underlying Watermill publisher.
func (w *watermillPublisher) Publish(ctx context.Context, topic string, event Event) error {
	rawPayload := event.RawPayload
	if len(rawPayload) == 0 {
		if event.RawObject != nil {
			if data, err := json.Marshal(event.RawObject); err == nil {
				rawPayload = data
			}
		}
	}
	if len(rawPayload) == 0 {
		data, err := json.Marshal(event)
		if err != nil {
			return err
		}
		rawPayload = data
	}

	protoPayload, err := proto.Marshal(&cloudv1.EventPayload{
		Provider: event.Provider,
		Name:     event.Name,
		Payload:  rawPayload,
	})
	if err != nil {
		return err
	}

	msg := message.NewMessage(watermill.NewUUID(), protoPayload)
	if msg.Metadata == nil {
		msg.Metadata = message.Metadata{}
	}
	if event.Provider != "" {
		msg.Metadata.Set("provider", event.Provider)
	}
	if event.Name != "" {
		msg.Metadata.Set("event", event.Name)
	}
	if event.RequestID != "" {
		msg.Metadata.Set("request_id", event.RequestID)
	}
	if event.StateID != "" {
		msg.Metadata.Set("state_id", event.StateID)
	}
	if event.InstallationID != "" {
		msg.Metadata.Set("installation_id", event.InstallationID)
	}
	if event.LogID != "" {
		msg.Metadata.Set("log_id", event.LogID)
	}
	msg.Metadata.Set("content_type", "application/x-protobuf")
	msg.Metadata.Set("schema", "cloud.v1.EventPayload")
	return w.publisher.Publish(topic, msg)
}

// Close closes the underlying Watermill publisher.
func (w *watermillPublisher) Close() error {
	if w.publisher == nil {
		return nil
	}
	err := w.publisher.Close()
	if w.closeFn != nil {
		return errors.Join(err, w.closeFn())
	}
	return err
}

// PublishForDrivers is a convenience method that calls Publish.
func (w *watermillPublisher) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return w.Publish(ctx, topic, event)
}

// publisherMux multiplexes events to multiple publishers.
type publisherMux struct {
	publishers     map[string]Publisher
	defaultDrivers []string
	retryAttempts  int
	retryDelay     time.Duration
	dlqDriver      string
}

// Publish sends an event to the default drivers.
func (m *publisherMux) Publish(ctx context.Context, topic string, event Event) error {
	return m.PublishForDrivers(ctx, topic, event, nil)
}

// PublishForDrivers sends an event to the specified drivers, or the default drivers if none are specified.
func (m *publisherMux) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	targets := drivers
	if len(targets) == 0 {
		targets = m.defaultDrivers
	}

	var err error
	for _, driver := range targets {
		normalized := strings.ToLower(driver)
		pub, ok := m.publishers[normalized]
		if !ok {
			err = errors.Join(err, fmt.Errorf("unknown driver %s", driver))
			continue
		}
		if publishErr := m.publishWithRetry(ctx, pub, topic, event); publishErr != nil {
			err = errors.Join(err, publishErr)
			if m.dlqDriver != "" && m.dlqDriver != normalized {
				if dlq, ok := m.publishers[m.dlqDriver]; ok {
					_ = m.publishWithRetry(ctx, dlq, topic, event)
				}
			}
		}
	}
	return err
}

func (m *publisherMux) publishWithRetry(ctx context.Context, pub Publisher, topic string, event Event) error {
	attempts := m.retryAttempts
	if attempts <= 0 {
		attempts = 1
	}
	delay := m.retryDelay
	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 && delay > 0 {
			time.Sleep(delay)
		}
		if err := pub.Publish(ctx, topic, event); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return lastErr
}

// Close closes all underlying publishers.
func (m *publisherMux) Close() error {
	var err error
	for _, pub := range m.publishers {
		err = errors.Join(err, pub.Close())
	}
	return err
}

func buildGoChannelPublisher(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
	pub := gochannel.NewGoChannel(
		gochannel.Config{
			OutputChannelBuffer:            cfg.GoChannel.OutputChannelBuffer,
			Persistent:                     cfg.GoChannel.Persistent,
			BlockPublishUntilSubscriberAck: cfg.GoChannel.BlockPublishUntilSubscriberAck,
		},
		logger,
	)
	return pub, nil, nil
}

func amqpConfigFromMode(url, mode string) (wmamaqp.Config, error) {
	switch strings.ToLower(mode) {
	case "", "durable_queue":
		return wmamaqp.NewDurableQueueConfig(url), nil
	case "nondurable_queue":
		return wmamaqp.NewNonDurableQueueConfig(url), nil
	case "durable_pubsub":
		return wmamaqp.NewDurablePubSubConfig(url, nil), nil
	case "nondurable_pubsub":
		return wmamaqp.NewNonDurablePubSubConfig(url, nil), nil
	default:
		return wmamaqp.Config{}, fmt.Errorf("unsupported amqp mode: %s", mode)
	}
}

func sqlSchemaAdapter(dialect string) (wmsql.SchemaAdapter, error) {
	switch strings.ToLower(dialect) {
	case "postgres", "postgresql":
		return postgresBinarySchema{}, nil
	case "mysql":
		return wmsql.DefaultMySQLSchema{}, nil
	default:
		return nil, fmt.Errorf("unsupported sql dialect: %s", dialect)
	}
}

func httpTargetURL(cfg HTTPConfig, topic string) (string, error) {
	switch strings.ToLower(cfg.Mode) {
	case "topic_url":
		if topic == "" {
			return "", fmt.Errorf("http topic url is empty")
		}
		return topic, nil
	case "base_url":
		if cfg.BaseURL == "" {
			return "", fmt.Errorf("http base_url is empty")
		}
		if topic == "" {
			return strings.TrimRight(cfg.BaseURL, "/"), nil
		}
		return strings.TrimRight(cfg.BaseURL, "/") + "/" + strings.TrimLeft(topic, "/"), nil
	default:
		return "", fmt.Errorf("unsupported http mode: %s", cfg.Mode)
	}
}

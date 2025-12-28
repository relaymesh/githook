package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	wmamaqp "github.com/ThreeDotsLabs/watermill-amqp/pkg/amqp"
	wmhttp "github.com/ThreeDotsLabs/watermill-http/v2/pkg/http"
	wmkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	stan "github.com/nats-io/stan.go"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, event Event) error
	PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error
	Close() error
}

type watermillPublisher struct {
	publisher message.Publisher
	closeFn   func() error
}

type PublisherFactory func(cfg WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error)

var publisherFactories = map[string]PublisherFactory{
	"gochannel": buildGoChannelPublisher,
}

func RegisterPublisherDriver(name string, factory PublisherFactory) {
	if name == "" || factory == nil {
		return
	}
	publisherFactories[strings.ToLower(name)] = factory
}

func NewPublisher(cfg WatermillConfig) (Publisher, error) {
	drivers := cfg.Drivers
	if len(drivers) == 0 && cfg.Driver != "" {
		drivers = []string{cfg.Driver}
	}
	if len(drivers) == 0 {
		drivers = []string{"gochannel"}
	}

	pubs := make(map[string]Publisher, len(drivers))
	for _, driver := range drivers {
		pub, err := newSinglePublisher(cfg, driver)
		if err != nil {
			for _, existing := range pubs {
				_ = existing.Close()
			}
			return nil, err
		}
		pubs[strings.ToLower(driver)] = pub
	}
	return &publisherMux{publishers: pubs, defaultDrivers: drivers}, nil
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
		pub, err := wmkafka.NewPublisher(cfg.Kafka.Brokers, wmkafka.DefaultMarshaler{}, nil, logger)
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
		pub, err := wmsql.NewPublisher(db, wmsql.PublisherConfig{
			SchemaAdapter:        schemaAdapter,
			AutoInitializeSchema: cfg.SQL.AutoInitializeSchema,
		}, logger)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		return &watermillPublisher{
			publisher: pub,
			closeFn:   db.Close,
		}, nil
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

func (w *watermillPublisher) Publish(ctx context.Context, topic string, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)
	return w.publisher.Publish(topic, msg)
}

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

func (w *watermillPublisher) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return w.Publish(ctx, topic, event)
}

type publisherMux struct {
	publishers     map[string]Publisher
	defaultDrivers []string
}

func (m *publisherMux) Publish(ctx context.Context, topic string, event Event) error {
	return m.PublishForDrivers(ctx, topic, event, nil)
}

func (m *publisherMux) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	targets := drivers
	if len(targets) == 0 {
		targets = m.defaultDrivers
	}

	var err error
	for _, driver := range targets {
		pub, ok := m.publishers[strings.ToLower(driver)]
		if !ok {
			err = errors.Join(err, fmt.Errorf("unknown driver %s", driver))
			continue
		}
		if publishErr := pub.Publish(ctx, topic, event); publishErr != nil {
			err = errors.Join(err, publishErr)
		}
	}
	return err
}

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
		return wmsql.DefaultPostgreSQLSchema{}, nil
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

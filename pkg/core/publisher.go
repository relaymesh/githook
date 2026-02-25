package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	amqpadapter "github.com/relaymesh/relaybus/sdk/amqp/go"
	relaycore "github.com/relaymesh/relaybus/sdk/core/go"
	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
	kafkaadapter "github.com/relaymesh/relaybus/sdk/kafka/go"
	natsadapter "github.com/relaymesh/relaybus/sdk/nats/go"
	"google.golang.org/protobuf/proto"

	cloudv1 "github.com/relaymesh/githook/pkg/gen/cloud/v1"
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

// relaybusPublisher wraps a Relaybus publisher and adapts it to the githook Publisher API.
type relaybusPublisher struct {
	publisher relaycore.Publisher
}

// PublisherFactory is a function that creates a custom publisher.
type PublisherFactory func(cfg RelaybusConfig) (Publisher, error)

var publisherFactories = map[string]PublisherFactory{}

// RegisterPublisherDriver registers a new publisher driver.
func RegisterPublisherDriver(name string, factory PublisherFactory) {
	if name == "" || factory == nil {
		return
	}
	publisherFactories[strings.ToLower(name)] = factory
}

// NewPublisher creates a new publisher based on the provided configuration.
// It can create multiple publishers if multiple drivers are configured.
func NewPublisher(cfg RelaybusConfig) (Publisher, error) {
	drivers := cfg.Drivers
	if len(drivers) == 0 && cfg.Driver != "" {
		drivers = []string{cfg.Driver}
	}

	pubs := make(map[string]Publisher, len(drivers))
	builtDrivers := make([]string, 0, len(drivers))
	for _, driver := range drivers {
		pub, err := retryPublisherBuild(func() (Publisher, error) {
			return newSinglePublisher(cfg, driver)
		})
		if err != nil {
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
		dlqDriver:      strings.ToLower(strings.TrimSpace(cfg.DLQDriver)),
	}, nil
}

// ValidatePublisherConfig validates driver config without connecting to brokers.
func ValidatePublisherConfig(cfg RelaybusConfig) error {
	drivers := cfg.Drivers
	if len(drivers) == 0 && cfg.Driver != "" {
		drivers = []string{cfg.Driver}
	}
	if len(drivers) == 0 {
		return nil
	}
	for _, driver := range drivers {
		if err := validatePublisherDriver(cfg, driver); err != nil {
			return err
		}
	}
	return nil
}

func newSinglePublisher(cfg RelaybusConfig, driver string) (Publisher, error) {
	switch strings.ToLower(driver) {
	case "kafka":
		brokers := cfg.Kafka.Brokers
		if len(brokers) == 0 && cfg.Kafka.Broker != "" {
			brokers = []string{cfg.Kafka.Broker}
		}
		if len(brokers) == 0 {
			return nil, fmt.Errorf("kafka brokers are required")
		}
		pub, err := relaycore.NewPublisher(relaycore.Config{
			Destination: "kafka",
			Retry:       relaybusRetryPolicy(cfg.PublishRetry),
			Logger:      relaycore.NopLogger{},
			Kafka: kafkaadapter.Config{
				Brokers:     brokers,
				TopicPrefix: cfg.Kafka.TopicPrefix,
			},
		})
		if err != nil {
			return nil, err
		}
		return &relaybusPublisher{publisher: pub}, nil
	case "nats":
		if cfg.NATS.URL == "" {
			return nil, fmt.Errorf("nats url is required")
		}
		pub, err := relaycore.NewPublisher(relaycore.Config{
			Destination: "nats",
			Retry:       relaybusRetryPolicy(cfg.PublishRetry),
			Logger:      relaycore.NopLogger{},
			NATS: natsadapter.Config{
				URL:           cfg.NATS.URL,
				SubjectPrefix: cfg.NATS.SubjectPrefix,
			},
		})
		if err != nil {
			return nil, err
		}
		return &relaybusPublisher{publisher: pub}, nil
	case "amqp":
		if cfg.AMQP.URL == "" {
			return nil, fmt.Errorf("amqp url is required")
		}
		pub, err := relaycore.NewPublisher(relaycore.Config{
			Destination: "amqp",
			Retry:       relaybusRetryPolicy(cfg.PublishRetry),
			Logger:      relaycore.NopLogger{},
			AMQP: amqpadapter.Config{
				URL:                cfg.AMQP.URL,
				Exchange:           cfg.AMQP.Exchange,
				Queue:              cfg.AMQP.Exchange,
				RoutingKeyTemplate: cfg.AMQP.RoutingKeyTemplate,
				Mandatory:          cfg.AMQP.Mandatory,
				Immediate:          cfg.AMQP.Immediate,
			},
		})
		if err != nil {
			return nil, err
		}
		return &relaybusPublisher{publisher: pub}, nil
	default:
		if factory, ok := publisherFactories[strings.ToLower(driver)]; ok {
			return factory(cfg)
		}
		return nil, fmt.Errorf("unsupported relaybus driver: %s", driver)
	}
}

func validatePublisherDriver(cfg RelaybusConfig, driver string) error {
	switch strings.ToLower(strings.TrimSpace(driver)) {
	case "amqp":
		if cfg.AMQP.URL == "" {
			return fmt.Errorf("amqp url is required")
		}
		return nil
	case "nats":
		if cfg.NATS.URL == "" {
			return fmt.Errorf("nats url is required")
		}
		return nil
	case "kafka":
		brokers := cfg.Kafka.Brokers
		if len(brokers) == 0 && cfg.Kafka.Broker != "" {
			brokers = []string{cfg.Kafka.Broker}
		}
		if len(brokers) == 0 {
			return fmt.Errorf("kafka brokers are required")
		}
		return nil
	default:
		if _, ok := publisherFactories[strings.ToLower(driver)]; ok {
			return nil
		}
		return fmt.Errorf("unsupported relaybus driver: %s", driver)
	}
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

// Publish sends an event to a topic using the underlying Relaybus publisher.
func (w *relaybusPublisher) Publish(ctx context.Context, topic string, event Event) error {
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

	metadata := make(map[string]string, 6)
	if event.Provider != "" {
		metadata["provider"] = event.Provider
	}
	if event.Name != "" {
		metadata["event"] = event.Name
	}
	if event.RequestID != "" {
		metadata["request_id"] = event.RequestID
	}
	if event.StateID != "" {
		metadata["state_id"] = event.StateID
	}
	if event.InstallationID != "" {
		metadata["installation_id"] = event.InstallationID
	}
	if event.ProviderInstanceKey != "" {
		metadata["provider_instance_key"] = event.ProviderInstanceKey
	}
	if event.LogID != "" {
		metadata["log_id"] = event.LogID
	}
	metadata["content_type"] = "application/x-protobuf"
	metadata["schema"] = "cloud.v1.EventPayload"
	msg := relaymessage.Message{
		Topic:       topic,
		Payload:     protoPayload,
		ContentType: "application/x-protobuf",
		Metadata:    metadata,
	}
	return w.publisher.Publish(ctx, topic, msg)
}

// PublishForDrivers is a convenience method that calls Publish.
func (w *relaybusPublisher) PublishForDrivers(ctx context.Context, topic string, event Event, drivers []string) error {
	return w.Publish(ctx, topic, event)
}

// Close closes the underlying Relaybus publisher.
func (w *relaybusPublisher) Close() error {
	if w.publisher == nil {
		return nil
	}
	return w.publisher.Close()
}

// publisherMux multiplexes events to multiple publishers.
type publisherMux struct {
	publishers     map[string]Publisher
	defaultDrivers []string
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
		if publishErr := pub.Publish(ctx, topic, event); publishErr != nil {
			err = errors.Join(err, publishErr)
			if m.dlqDriver != "" && m.dlqDriver != normalized {
				if dlq, ok := m.publishers[m.dlqDriver]; ok {
					_ = dlq.Publish(ctx, topic, event)
				}
			}
		}
	}
	return err
}

// Close closes all underlying publishers.
func (m *publisherMux) Close() error {
	var err error
	for _, pub := range m.publishers {
		err = errors.Join(err, pub.Close())
	}
	return err
}

func relaybusRetryPolicy(cfg PublishRetryConfig) relaycore.RetryPolicy {
	attempts := cfg.Attempts
	if attempts <= 0 {
		attempts = 1
	}
	delay := time.Duration(cfg.DelayMS) * time.Millisecond
	if delay < 0 {
		delay = 0
	}
	return relaycore.RetryPolicy{
		MaxAttempts: attempts,
		BaseDelay:   delay,
		MaxDelay:    delay,
	}
}

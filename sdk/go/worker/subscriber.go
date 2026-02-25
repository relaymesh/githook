package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	amqpadapter "github.com/relaymesh/relaybus/sdk/amqp/go"
	relaymessage "github.com/relaymesh/relaybus/sdk/core/go/message"
	kafkaadapter "github.com/relaymesh/relaybus/sdk/kafka/go"
	natsadapter "github.com/relaymesh/relaybus/sdk/nats/go"
)

// MessageHandler receives relaybus messages for a subscription.
type MessageHandler func(ctx context.Context, msg relaymessage.Message) error

// Subscriber represents a Relaybus subscriber.
type Subscriber interface {
	Start(ctx context.Context, topic string, handler MessageHandler) error
	Close() error
}

// NewFromConfig creates a new worker from a subscriber configuration.
func NewFromConfig(cfg SubscriberConfig, opts ...Option) (*Worker, error) {
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, WithSubscriber(sub))
	return New(opts...), nil
}

// BuildSubscriber creates a Relaybus subscriber from a configuration.
// It can create a single subscriber or a multi-subscriber that combines multiple drivers.
func BuildSubscriber(cfg SubscriberConfig) (Subscriber, error) {
	drivers := uniqueStrings(cfg.Drivers)
	if cfg.Driver != "" {
		drivers = append(drivers, cfg.Driver)
		drivers = uniqueStrings(drivers)
	}
	if len(drivers) == 0 {
		return nil, errors.New("at least one driver is required")
	}
	if len(drivers) == 1 {
		return newSingleSubscriber(cfg, drivers[0])
	}

	subs := make([]namedSubscriber, 0, len(drivers))
	for _, driver := range drivers {
		sub, err := newSingleSubscriber(cfg, driver)
		if err != nil {
			return nil, err
		}
		subs = append(subs, namedSubscriber{driver: driver, sub: sub})
	}

	return &multiSubscriber{
		subscribers: subs,
	}, nil
}

func newSingleSubscriber(cfg SubscriberConfig, driver string) (Subscriber, error) {
	driver = strings.ToLower(strings.TrimSpace(driver))
	if !isSubscriberDriverSupported(driver) {
		return nil, fmt.Errorf("unsupported subscriber driver: %s", driver)
	}
	return &relaybusSubscriber{driver: driver, cfg: cfg}, nil
}

type relaybusSubscriber struct {
	driver string
	cfg    SubscriberConfig
}

func (s *relaybusSubscriber) Start(ctx context.Context, topic string, handler MessageHandler) error {
	if handler == nil {
		return errors.New("handler is required")
	}
	switch s.driver {
	case "amqp":
		if s.cfg.AMQP.URL == "" {
			return errors.New("amqp url is required")
		}
		sub, err := amqpadapter.NewSubscriber(amqpadapter.SubscriberConfig{
			URL:                s.cfg.AMQP.URL,
			Exchange:           s.cfg.AMQP.Exchange,
			RoutingKeyTemplate: s.cfg.AMQP.RoutingKeyTemplate,
			Queue:              s.cfg.AMQP.Exchange,
			AutoAck:            s.cfg.AMQP.AutoAck,
			MaxMessages:        s.cfg.AMQP.MaxMessages,
			Handler:            handler,
		})
		if err != nil {
			return err
		}
		return sub.Start(ctx, topic)
	case "nats":
		if s.cfg.NATS.URL == "" {
			return errors.New("nats url is required")
		}
		sub, err := natsadapter.NewSubscriber(natsadapter.SubscriberConfig{
			URL:           s.cfg.NATS.URL,
			SubjectPrefix: s.cfg.NATS.SubjectPrefix,
			MaxMessages:   s.cfg.NATS.MaxMessages,
			Handler:       handler,
		})
		if err != nil {
			return err
		}
		return sub.Start(ctx, topic)
	case "kafka":
		sub, err := kafkaadapter.NewSubscriber(kafkaadapter.SubscriberConfig{
			Brokers:     s.cfg.Kafka.Brokers,
			Broker:      s.cfg.Kafka.Broker,
			GroupID:     s.cfg.Kafka.GroupID,
			MaxMessages: s.cfg.Kafka.MaxMessages,
			Handler:     handler,
		})
		if err != nil {
			return err
		}
		kafkaTopic := topic
		if s.cfg.Kafka.TopicPrefix != "" {
			kafkaTopic = s.cfg.Kafka.TopicPrefix + topic
		}
		return sub.Start(ctx, kafkaTopic)
	default:
		return fmt.Errorf("unsupported subscriber driver: %s", s.driver)
	}
}

func (s *relaybusSubscriber) Close() error {
	return nil
}

type multiSubscriber struct {
	subscribers []namedSubscriber
}

type namedSubscriber struct {
	driver string
	sub    Subscriber
}

func (m *multiSubscriber) Start(ctx context.Context, topic string, handler MessageHandler) error {
	if len(m.subscribers) == 0 {
		return errors.New("no subscribers configured")
	}
	if handler == nil {
		return errors.New("handler is required")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, len(m.subscribers))
	var wg sync.WaitGroup
	wg.Add(len(m.subscribers))

	for _, entry := range m.subscribers {
		entry := entry
		go func() {
			defer wg.Done()
			errCh <- entry.sub.Start(ctx, topic, func(ctx context.Context, msg relaymessage.Message) error {
				if msg.Metadata == nil {
					msg.Metadata = map[string]string{}
				}
				if entry.driver != "" {
					msg.Metadata[MetadataKeyDriver] = entry.driver
				}
				return handler(ctx, msg)
			})
		}()
	}

	go func() {
		wg.Wait()
		close(errCh)
	}()

	var err error
	for subErr := range errCh {
		if subErr != nil {
			err = errors.Join(err, subErr)
			cancel()
		}
	}
	return err
}

func (m *multiSubscriber) Close() error {
	var firstErr error
	for _, entry := range m.subscribers {
		if err := entry.sub.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func isSubscriberDriverSupported(driver string) bool {
	switch strings.ToLower(driver) {
	case "amqp", "nats", "kafka":
		return true
	default:
		return false
	}
}

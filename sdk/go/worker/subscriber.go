package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ThreeDotsLabs/watermill"
	wmamaqp "github.com/ThreeDotsLabs/watermill-amqp/pkg/amqp"
	wmkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	stan "github.com/nats-io/stan.go"
)

// NewFromConfig creates a new worker from a subscriber configuration.
func NewFromConfig(cfg SubscriberConfig, opts ...Option) (*Worker, error) {
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, WithSubscriber(sub))
	return New(opts...), nil
}

// BuildSubscriber creates a new Watermill subscriber from a configuration.
// It can create a single subscriber or a multi-subscriber that combines multiple drivers.
func BuildSubscriber(cfg SubscriberConfig) (message.Subscriber, error) {
	logger := watermill.NewStdLogger(false, false)

	if len(cfg.Drivers) > 0 {
		return buildMultiSubscriber(cfg, logger)
	}

	driver := cfg.Driver
	if driver == "" {
		driver = "gochannel"
	}

	return retrySubscriberBuild(func() (message.Subscriber, error) {
		return buildSingleSubscriber(cfg, logger, driver)
	})
}

func buildMultiSubscriber(cfg SubscriberConfig, logger watermill.LoggerAdapter) (message.Subscriber, error) {
	drivers := uniqueStrings(cfg.Drivers)
	if cfg.Driver != "" {
		drivers = append(drivers, cfg.Driver)
		drivers = uniqueStrings(drivers)
	}
	if len(drivers) == 0 {
		return nil, errors.New("at least one driver is required")
	}

	subs := make([]namedSubscriber, 0, len(drivers))
	for _, driver := range drivers {
		if !isSubscriberDriverSupported(driver) {
			logger.Info("skipping unsupported subscriber driver", watermill.LogFields{
				"driver": driver,
			})
			continue
		}
		sub, err := retrySubscriberBuild(func() (message.Subscriber, error) {
			return buildSingleSubscriber(cfg, logger, driver)
		})
		if err != nil {
			logger.Error("subscriber init failed, skipping driver", err, watermill.LogFields{
				"driver": driver,
			})
			continue
		}
		subs = append(subs, namedSubscriber{driver: driver, sub: sub})
	}

	if len(subs) == 0 {
		return nil, errors.New("no supported subscriber drivers configured")
	}

	return &multiSubscriber{
		subscribers: subs,
		bufferSize:  cfg.GoChannel.OutputChannelBuffer,
	}, nil
}

func buildSingleSubscriber(cfg SubscriberConfig, logger watermill.LoggerAdapter, driver string) (message.Subscriber, error) {
	switch strings.ToLower(driver) {
	case "gochannel":
		return gochannel.NewGoChannel(gochannel.Config{
			OutputChannelBuffer:            cfg.GoChannel.OutputChannelBuffer,
			Persistent:                     cfg.GoChannel.Persistent,
			BlockPublishUntilSubscriberAck: cfg.GoChannel.BlockPublishUntilSubscriberAck,
		}, logger), nil
	case "amqp":
		if cfg.AMQP.URL == "" {
			return nil, errors.New("amqp url is required")
		}
		amqpCfg, err := amqpSubscriberConfigFromMode(cfg.AMQP.URL, cfg.AMQP.Mode)
		if err != nil {
			return nil, err
		}
		return retrySubscriberBuild(func() (message.Subscriber, error) {
			return wmamaqp.NewSubscriber(amqpCfg, logger)
		})
	case "nats":
		if cfg.NATS.ClusterID == "" || cfg.NATS.ClientID == "" {
			return nil, errors.New("nats cluster_id and client_id are required")
		}
		clientID := cfg.NATS.ClientID
		if cfg.NATS.ClientIDSuffix != "" {
			clientID = clientID + cfg.NATS.ClientIDSuffix
		}
		natsCfg := wmnats.StreamingSubscriberConfig{
			ClusterID:   cfg.NATS.ClusterID,
			ClientID:    clientID,
			DurableName: cfg.NATS.Durable,
			Unmarshaler: wmnats.GobMarshaler{},
		}
		if cfg.NATS.URL != "" {
			natsCfg.StanOptions = append(natsCfg.StanOptions, stan.NatsURL(cfg.NATS.URL))
		}
		return retrySubscriberBuild(func() (message.Subscriber, error) {
			return wmnats.NewStreamingSubscriber(natsCfg, logger)
		})
	case "kafka":
		if len(cfg.Kafka.Brokers) == 0 {
			return nil, errors.New("kafka brokers are required")
		}
		return wmkafka.NewSubscriber(wmkafka.SubscriberConfig{
			Brokers:       cfg.Kafka.Brokers,
			ConsumerGroup: cfg.Kafka.ConsumerGroup,
		}, nil, wmkafka.DefaultMarshaler{}, logger)
	case "sql":
		if cfg.SQL.Driver == "" || cfg.SQL.DSN == "" {
			return nil, errors.New("sql driver and dsn are required")
		}
		schemaAdapter, offsetsAdapter, err := sqlAdapters(cfg.SQL.Dialect)
		if err != nil {
			return nil, err
		}
		initialize := cfg.SQL.InitializeSchema || cfg.SQL.AutoInitializeSchema
		sub, err := retrySubscriberBuild(func() (message.Subscriber, error) {
			db, err := sql.Open(cfg.SQL.Driver, cfg.SQL.DSN)
			if err != nil {
				return nil, err
			}
			sub, err := wmsql.NewSubscriber(db, wmsql.SubscriberConfig{
				ConsumerGroup:    cfg.SQL.ConsumerGroup,
				SchemaAdapter:    schemaAdapter,
				OffsetsAdapter:   offsetsAdapter,
				InitializeSchema: initialize,
			}, logger)
			if err != nil {
				_ = db.Close()
				return nil, err
			}
			return &closingSubscriber{Subscriber: sub, closeFn: db.Close}, nil
		})
		if err != nil {
			return nil, err
		}
		return sub, nil
	case "riverqueue":
		return newRiverQueueSubscriber(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported subscriber driver: %s", driver)
	}
}

func retrySubscriberBuild(build func() (message.Subscriber, error)) (message.Subscriber, error) {
	const attempts = 10
	const delay = 2 * time.Second

	var lastErr error
	for i := 0; i < attempts; i++ {
		sub, err := build()
		if err == nil {
			return sub, nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	return nil, lastErr
}

type closingSubscriber struct {
	message.Subscriber
	closeFn func() error
}

func (c *closingSubscriber) Close() error {
	err := c.Subscriber.Close()
	if c.closeFn != nil {
		if closeErr := c.closeFn(); closeErr != nil {
			if err == nil {
				return closeErr
			}
			return fmt.Errorf("%v; %w", err, closeErr)
		}
	}
	return err
}

type multiSubscriber struct {
	subscribers []namedSubscriber
	bufferSize  int64
}

type namedSubscriber struct {
	driver string
	sub    message.Subscriber
}

func (m *multiSubscriber) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	if len(m.subscribers) == 0 {
		return nil, errors.New("no subscribers configured")
	}

	buffer := m.bufferSize
	if buffer <= 0 {
		buffer = 64
	}
	out := make(chan *message.Message, buffer)

	var wg sync.WaitGroup
	wg.Add(len(m.subscribers))

	for _, entry := range m.subscribers {
		ch, err := entry.sub.Subscribe(ctx, topic)
		if err != nil {
			for _, existing := range m.subscribers {
				_ = existing.sub.Close()
			}
			return nil, err
		}
		driver := entry.driver
		go func(ch <-chan *message.Message, driver string) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-ch:
					if !ok {
						return
					}
					if msg.Metadata == nil {
						msg.Metadata = message.Metadata{}
					}
					msg.Metadata.Set(MetadataKeyDriver, driver)
					out <- msg
				}
			}
		}(ch, driver)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out, nil
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

func amqpSubscriberConfigFromMode(url, mode string) (wmamaqp.Config, error) {
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

func sqlAdapters(dialect string) (wmsql.SchemaAdapter, wmsql.OffsetsAdapter, error) {
	switch strings.ToLower(dialect) {
	case "postgres", "postgresql":
		return postgresBinarySchema{}, wmsql.DefaultPostgreSQLOffsetsAdapter{}, nil
	case "mysql":
		return wmsql.DefaultMySQLSchema{}, wmsql.DefaultMySQLOffsetsAdapter{}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported sql dialect: %s", dialect)
	}
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
	case "gochannel", "amqp", "nats", "kafka", "sql", "riverqueue":
		return true
	default:
		return false
	}
}

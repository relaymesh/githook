package worker

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

func NewFromConfig(cfg SubscriberConfig, opts ...Option) (*Worker, error) {
	sub, err := BuildSubscriber(cfg)
	if err != nil {
		return nil, err
	}
	opts = append(opts, WithSubscriber(sub))
	return New(opts...), nil
}

func BuildSubscriber(cfg SubscriberConfig) (message.Subscriber, error) {
	logger := watermill.NewStdLogger(false, false)

	driver := cfg.Driver
	if driver == "" && len(cfg.Drivers) > 0 {
		driver = cfg.Drivers[0]
	}
	if driver == "" {
		driver = "gochannel"
	}

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
		return retrySubscriber(func() (message.Subscriber, error) {
			return wmamaqp.NewSubscriber(amqpCfg, logger)
		})
	case "nats":
		if cfg.NATS.ClusterID == "" || cfg.NATS.ClientID == "" {
			return nil, errors.New("nats cluster_id and client_id are required")
		}
		natsCfg := wmnats.StreamingSubscriberConfig{
			ClusterID:   cfg.NATS.ClusterID,
			ClientID:    cfg.NATS.ClientID,
			DurableName: cfg.NATS.Durable,
			Unmarshaler: wmnats.GobMarshaler{},
		}
		if cfg.NATS.URL != "" {
			natsCfg.StanOptions = append(natsCfg.StanOptions, stan.NatsURL(cfg.NATS.URL))
		}
		return retrySubscriber(func() (message.Subscriber, error) {
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
		sub, err := retrySubscriber(func() (message.Subscriber, error) {
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
	default:
		return nil, fmt.Errorf("unsupported subscriber driver: %s", driver)
	}
}

func retrySubscriber(build func() (message.Subscriber, error)) (message.Subscriber, error) {
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
		return wmsql.DefaultPostgreSQLSchema{}, wmsql.DefaultPostgreSQLOffsetsAdapter{}, nil
	case "mysql":
		return wmsql.DefaultMySQLSchema{}, wmsql.DefaultMySQLOffsetsAdapter{}, nil
	default:
		return nil, nil, fmt.Errorf("unsupported sql dialect: %s", dialect)
	}
}

package internal

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	wmamaqp "github.com/ThreeDotsLabs/watermill-amqp/pkg/amqp"
	wmkafka "github.com/ThreeDotsLabs/watermill-kafka/pkg/kafka"
	wmnats "github.com/ThreeDotsLabs/watermill-nats/pkg/nats"
	wmsql "github.com/ThreeDotsLabs/watermill-sql/pkg/sql"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	stan "github.com/nats-io/stan.go"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, event Event) error
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
	logger := watermill.NewStdLogger(false, false)

	switch strings.ToLower(cfg.Driver) {
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
		if factory, ok := publisherFactories[strings.ToLower(cfg.Driver)]; ok {
			pub, closeFn, err := factory(cfg, logger)
			if err != nil {
				return nil, err
			}
			return &watermillPublisher{publisher: pub, closeFn: closeFn}, nil
		}
		return nil, fmt.Errorf("unsupported watermill driver: %s", cfg.Driver)
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

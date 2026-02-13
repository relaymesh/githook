package worker

// SubscriberConfig holds the configuration for a Watermill subscriber.
type SubscriberConfig struct {
	Driver  string   `yaml:"driver"`
	Drivers []string `yaml:"drivers"`

	GoChannel  GoChannelConfig  `yaml:"gochannel"`
	Kafka      KafkaConfig      `yaml:"kafka"`
	NATS       NATSConfig       `yaml:"nats"`
	AMQP       AMQPConfig       `yaml:"amqp"`
	SQL        SQLConfig        `yaml:"sql"`
	RiverQueue RiverQueueConfig `yaml:"riverqueue"`
}

// GoChannelConfig holds configuration for the GoChannel pub/sub.
type GoChannelConfig struct {
	OutputChannelBuffer            int64 `yaml:"output_buffer"`
	Persistent                     bool  `yaml:"persistent"`
	BlockPublishUntilSubscriberAck bool  `yaml:"block_publish_until_subscriber_ack"`
}

// KafkaConfig holds configuration for the Kafka pub/sub.
type KafkaConfig struct {
	Brokers       []string `yaml:"brokers"`
	ConsumerGroup string   `yaml:"consumer_group"`
}

// NATSConfig holds configuration for the NATS pub/sub.
type NATSConfig struct {
	ClusterID      string `yaml:"cluster_id"`
	ClientID       string `yaml:"client_id"`
	ClientIDSuffix string `yaml:"client_id_suffix"`
	URL            string `yaml:"url"`
	Durable        string `yaml:"durable"`
}

// AMQPConfig holds configuration for the AMQP pub/sub.
type AMQPConfig struct {
	URL  string `yaml:"url"`
	Mode string `yaml:"mode"`
}

// SQLConfig holds configuration for the SQL pub/sub.
type SQLConfig struct {
	Driver               string `yaml:"driver"`
	DSN                  string `yaml:"dsn"`
	Dialect              string `yaml:"dialect"`
	ConsumerGroup        string `yaml:"consumer_group"`
	InitializeSchema     bool   `yaml:"initialize_schema"`
	AutoInitializeSchema bool   `yaml:"auto_initialize_schema"`
}

// RiverQueueConfig holds configuration for RiverQueue subscribers.
type RiverQueueConfig struct {
	Driver      string   `yaml:"driver"`
	DSN         string   `yaml:"dsn"`
	Table       string   `yaml:"table"`
	Queue       string   `yaml:"queue"`
	Kind        string   `yaml:"kind"`
	MaxAttempts int      `yaml:"max_attempts"`
	Priority    int      `yaml:"priority"`
	Tags        []string `yaml:"tags"`
	MaxWorkers  int      `yaml:"max_workers"`
}

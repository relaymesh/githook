package worker

type SubscriberConfig struct {
	Driver  string   `yaml:"driver"`
	Drivers []string `yaml:"drivers"`

	GoChannel GoChannelConfig `yaml:"gochannel"`
	Kafka     KafkaConfig     `yaml:"kafka"`
	NATS      NATSConfig      `yaml:"nats"`
	AMQP      AMQPConfig      `yaml:"amqp"`
	SQL       SQLConfig       `yaml:"sql"`
}

type GoChannelConfig struct {
	OutputChannelBuffer            int64 `yaml:"output_buffer"`
	Persistent                     bool  `yaml:"persistent"`
	BlockPublishUntilSubscriberAck bool  `yaml:"block_publish_until_subscriber_ack"`
}

type KafkaConfig struct {
	Brokers       []string `yaml:"brokers"`
	ConsumerGroup string   `yaml:"consumer_group"`
}

type NATSConfig struct {
	ClusterID string `yaml:"cluster_id"`
	ClientID  string `yaml:"client_id"`
	URL       string `yaml:"url"`
	Durable   string `yaml:"durable"`
}

type AMQPConfig struct {
	URL  string `yaml:"url"`
	Mode string `yaml:"mode"`
}

type SQLConfig struct {
	Driver               string `yaml:"driver"`
	DSN                  string `yaml:"dsn"`
	Dialect              string `yaml:"dialect"`
	ConsumerGroup        string `yaml:"consumer_group"`
	InitializeSchema     bool   `yaml:"initialize_schema"`
	AutoInitializeSchema bool   `yaml:"auto_initialize_schema"`
}

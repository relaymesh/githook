package worker

// SubscriberConfig holds the configuration for a Relaybus subscriber.
type SubscriberConfig struct {
	Driver  string   `yaml:"driver"`
	Drivers []string `yaml:"drivers"`

	Kafka KafkaConfig `yaml:"kafka"`
	NATS  NATSConfig  `yaml:"nats"`
	AMQP  AMQPConfig  `yaml:"amqp"`
}

// KafkaConfig holds configuration for the Kafka pub/sub.
type KafkaConfig struct {
	Brokers     []string `yaml:"brokers"`
	Broker      string   `yaml:"broker"`
	GroupID     string   `yaml:"group_id"`
	TopicPrefix string   `yaml:"topic_prefix"`
	MaxMessages int      `yaml:"max_messages"`
}

// NATSConfig holds configuration for the NATS pub/sub.
type NATSConfig struct {
	URL           string `yaml:"url"`
	SubjectPrefix string `yaml:"subject_prefix"`
	MaxMessages   int    `yaml:"max_messages"`
}

// AMQPConfig holds configuration for the AMQP pub/sub.
type AMQPConfig struct {
	URL                string `yaml:"url"`
	Exchange           string `yaml:"exchange"`
	RoutingKeyTemplate string `yaml:"routing_key_template"`
	Queue              string `yaml:"queue"`
	AutoAck            bool   `yaml:"auto_ack"`
	MaxMessages        int    `yaml:"max_messages"`
}

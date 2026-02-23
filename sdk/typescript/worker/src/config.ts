export type Driver = "amqp" | "kafka" | "nats";

export interface SubscriberConfig {
  driver?: Driver;
  drivers?: Driver[];
  amqp?: AmqpConfig;
  kafka?: KafkaConfig;
  nats?: NatsConfig;
}

export interface AmqpConfig {
  url: string;
  exchange?: string;
  routingKeyTemplate?: string;
  queue?: string;
  autoAck?: boolean;
  maxMessages?: number;
}

export interface KafkaConfig {
  brokers?: string[];
  broker?: string;
  groupId?: string;
  topicPrefix?: string;
  maxMessages?: number;
}

export interface NatsConfig {
  url: string;
  subjectPrefix?: string;
  maxMessages?: number;
}

declare module "@relaymesh/relaybus-amqp" {
  export const AmqpSubscriber: {
    connect: (cfg: unknown) => Promise<{
      start: (topic: string) => Promise<void>;
      close: () => Promise<void>;
    }>;
  };
}

declare module "@relaymesh/relaybus-kafka" {
  export const KafkaSubscriber: {
    connect: (cfg: unknown) => Promise<{
      start: (topic: string) => Promise<void>;
      close: () => Promise<void>;
    }>;
  };
}

declare module "@relaymesh/relaybus-nats" {
  export const NatsSubscriber: {
    connect: (cfg: unknown) => Promise<{
      start: (topic: string) => Promise<void>;
      close: () => Promise<void>;
    }>;
  };
}

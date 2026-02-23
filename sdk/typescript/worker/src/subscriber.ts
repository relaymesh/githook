import { AmqpSubscriber } from "@relaymesh/relaybus-amqp";
import { KafkaSubscriber } from "@relaymesh/relaybus-kafka";
import { NatsSubscriber } from "@relaymesh/relaybus-nats";

import type { Driver, SubscriberConfig } from "./config.js";
import { MetadataKeyDriver } from "./metadata.js";
import type { MessageHandler, RelaybusMessage } from "./types.js";

export interface Subscriber {
  start(topic: string, handler: MessageHandler): Promise<void>;
  close(): Promise<void>;
}

export function buildSubscriber(cfg: SubscriberConfig): Subscriber {
  const drivers = collectDrivers(cfg);
  if (drivers.length === 0) {
    throw new Error("at least one driver is required");
  }
  if (drivers.length === 1) {
    return new RelaybusSubscriber(drivers[0], cfg);
  }

  const entries = drivers.map((driver) => ({
    driver,
    sub: new RelaybusSubscriber(driver, cfg),
  }));
  return new MultiSubscriber(entries);
}

class RelaybusSubscriber implements Subscriber {
  private readonly subs = new Set<{ close: () => Promise<void> }>();

  constructor(private readonly driver: Driver, private readonly cfg: SubscriberConfig) {}

  async start(topic: string, handler: MessageHandler): Promise<void> {
    if (!handler) {
      throw new Error("handler is required");
    }
    switch (this.driver) {
      case "amqp":
        await this.startAmqp(topic, handler);
        return;
      case "kafka":
        await this.startKafka(topic, handler);
        return;
      case "nats":
        await this.startNats(topic, handler);
        return;
      default:
        throw new Error(`unsupported driver: ${this.driver}`);
    }
  }

  async close(): Promise<void> {
    let firstError: Error | undefined;
    for (const sub of this.subs) {
      try {
        await sub.close();
      } catch (err) {
        if (!firstError) {
          firstError = err instanceof Error ? err : new Error(String(err));
        }
      }
    }
    this.subs.clear();
    if (firstError) {
      throw firstError;
    }
  }

  private async startAmqp(topic: string, handler: MessageHandler): Promise<void> {
    const cfg = this.cfg.amqp;
    if (!cfg || !cfg.url) {
      throw new Error("amqp url is required");
    }
    const sub = await AmqpSubscriber.connect({
      url: cfg.url,
      exchange: cfg.exchange,
      routingKeyTemplate: cfg.routingKeyTemplate,
      queue: cfg.queue,
      autoAck: cfg.autoAck,
      maxMessages: cfg.maxMessages,
      onMessage: (msg: RelaybusMessage) => handler(msg),
    });
    this.subs.add(sub);
    try {
      await sub.start(topic);
    } finally {
      this.subs.delete(sub);
    }
  }

  private async startKafka(topic: string, handler: MessageHandler): Promise<void> {
    const cfg = this.cfg.kafka;
    const brokers = cfg?.brokers?.length ? cfg.brokers : cfg?.broker ? [cfg.broker] : [];
    if (!brokers.length) {
      throw new Error("kafka brokers are required");
    }
    const sub = await KafkaSubscriber.connect({
      brokers,
      groupId: cfg?.groupId,
      maxMessages: cfg?.maxMessages,
      onMessage: (msg: RelaybusMessage) => handler(msg),
    });
    this.subs.add(sub);
    try {
      await sub.start(resolveKafkaTopic(topic, cfg));
    } finally {
      this.subs.delete(sub);
    }
  }

  private async startNats(topic: string, handler: MessageHandler): Promise<void> {
    const cfg = this.cfg.nats;
    if (!cfg || !cfg.url) {
      throw new Error("nats url is required");
    }
    const sub = await NatsSubscriber.connect({
      url: cfg.url,
      subjectPrefix: cfg.subjectPrefix,
      maxMessages: cfg.maxMessages,
      onMessage: (msg: RelaybusMessage) => handler(msg),
    });
    this.subs.add(sub);
    try {
      await sub.start(topic);
    } finally {
      this.subs.delete(sub);
    }
  }
}

class MultiSubscriber implements Subscriber {
  constructor(private readonly entries: Array<{ driver: Driver; sub: Subscriber }>) {}

  async start(topic: string, handler: MessageHandler): Promise<void> {
    if (!handler) {
      throw new Error("handler is required");
    }
    await Promise.all(
      this.entries.map((entry) =>
        entry.sub.start(topic, async (msg) => {
          const metadata = { ...(msg.metadata ?? {}) };
          metadata[MetadataKeyDriver] = entry.driver;
          msg.metadata = metadata;
          await handler(msg);
        }),
      ),
    );
  }

  async close(): Promise<void> {
    let firstError: Error | undefined;
    for (const entry of this.entries) {
      try {
        await entry.sub.close();
      } catch (err) {
        if (!firstError) {
          firstError = err instanceof Error ? err : new Error(String(err));
        }
      }
    }
    if (firstError) {
      throw firstError;
    }
  }
}

function collectDrivers(cfg: SubscriberConfig): Driver[] {
  const drivers = new Set<Driver>();
  const append = (value?: Driver) => {
    const normalized = (value ?? "").toLowerCase().trim() as Driver;
    if (normalized) {
      drivers.add(normalized);
    }
  };
  append(cfg.driver);
  if (Array.isArray(cfg.drivers)) {
    for (const driver of cfg.drivers) {
      append(driver);
    }
  }
  return Array.from(drivers);
}

function resolveKafkaTopic(topic: string, cfg?: SubscriberConfig["kafka"]): string {
  const prefix = cfg?.topicPrefix ?? "";
  if (!prefix) {
    return topic;
  }
  return `${prefix}${topic}`;
}

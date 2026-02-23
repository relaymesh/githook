import type { SubscriberConfig } from "./config.js";

export function subscriberConfigFromDriver(driver: string, raw: string): SubscriberConfig {
  const normalized = (driver ?? "").trim().toLowerCase();
  if (!normalized) {
    throw new Error("driver is required");
  }
  const cfg: SubscriberConfig = { driver: normalized as SubscriberConfig["driver"] };
  applyDriverConfig(cfg, normalized, raw);
  return cfg;
}

function applyDriverConfig(cfg: SubscriberConfig, name: string, raw: string): void {
  const payload = parseJSON(raw);
  if (!payload) {
    return;
  }
  switch (name) {
    case "amqp":
      cfg.amqp = payload as SubscriberConfig["amqp"];
      return;
    case "nats":
      cfg.nats = payload as SubscriberConfig["nats"];
      return;
    case "kafka":
      cfg.kafka = payload as SubscriberConfig["kafka"];
      return;
    default:
      throw new Error(`unsupported driver: ${name}`);
  }
}

function parseJSON(raw: string): unknown | undefined {
  const trimmed = (raw ?? "").trim();
  if (!trimmed) {
    return undefined;
  }
  return JSON.parse(trimmed);
}

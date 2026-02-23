# Driver Configuration

githook uses [Relaybus](https://github.com/relaymesh/relaybus) for publishing and subscribing. Drivers are configured via the CLI and stored on the server per tenant.

Driver config files contain only the driver-specific fields (no `relaybus:` wrapper). The CLI expects
YAML and converts it to the JSON payload required by the API.

## Supported Drivers

- `amqp`
- `nats`
- `kafka`

## AMQP (RabbitMQ)

Publisher fields:
- `url` (required)
- `exchange`
- `routing_key_template` (default: `{topic}`)
- `mandatory`
- `immediate`

Worker-only fields:
- `queue`
- `auto_ack`
- `max_messages`

Example:
```yaml
url: amqp://guest:guest@localhost:5672/
exchange: githook.events
routing_key_template: "{topic}"
queue: githook.worker
auto_ack: false
```

## NATS

Fields:
- `url` (required)
- `subject_prefix`
- `max_messages` (worker only)

Example:
```yaml
url: nats://localhost:4222
subject_prefix: githook
```

## Kafka

Fields:
- `broker` or `brokers` (required)
- `topic_prefix`
- `group_id` (worker only)
- `max_messages` (worker only)

Example:
```yaml
brokers:
  - localhost:29092
topic_prefix: githook.
group_id: githook-worker
```

## CLI Usage

```sh
githook --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.yaml
githook --endpoint http://localhost:8080 drivers set --name nats --config-file nats.yaml
githook --endpoint http://localhost:8080 drivers set --name kafka --config-file kafka.yaml
```

## Publish Retry/DLQ

Server-wide publish retry settings live in `config.yaml`:
```yaml
relaybus:
  publish_retry:
    attempts: 3
    delay_ms: 500
  dlq_driver: amqp
```

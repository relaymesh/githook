# Driver Configuration

Drivers describe how githook publishes events and how workers subscribe to them.
Driver configs are stored on the server per tenant and managed via the CLI.

Driver config files contain only driver fields (no `relaybus:` wrapper). The CLI reads YAML and sends JSON to the API.

## Supported drivers

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
```

Ensure the exchange exists in your broker.

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

## CLI usage

```bash
githook --endpoint http://localhost:8080 drivers create --name amqp --config-file amqp.yaml
githook --endpoint http://localhost:8080 drivers update --name amqp --config-file amqp.yaml
githook --endpoint http://localhost:8080 drivers list
githook --endpoint http://localhost:8080 drivers get --name amqp
githook --endpoint http://localhost:8080 drivers delete --name amqp
```

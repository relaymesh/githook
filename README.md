# Githooks

Config-driven webhook router for GitHub (with GitLab/Bitbucket planned). It normalizes inbound webhook events, evaluates them against YAML rules, and publishes matching events to Watermill topics for downstream consumers.

## Features
- Typed webhook parsing via go-playground/webhooks
- Provider-agnostic normalized event model
- Rule-based routing via govaluate
- Watermill-backed publishing (gochannel, Kafka, NATS Streaming, AMQP, SQL)
- Stateless by default

## Architecture
Webhook Provider -> go-playground/webhooks -> Adapter -> Normalized Event -> Rule Engine -> Watermill Publisher

## Quickstart
1) Configure secrets and rules in `app.yaml` and `config.yaml`.
2) Export any secrets referenced by env vars.
3) Run:
```bash
go run main.go
```

Example:
```bash
export GITHUB_WEBHOOK_SECRET=devsecret
go run main.go
```

Then send GitHub webhooks to:
`http://localhost:8080/webhooks/github`

## Local Dependencies (Docker Compose)
Spin up local brokers and databases:
```bash
docker-compose up -d
```

Useful endpoints:
- RabbitMQ UI: http://localhost:15672 (guest/guest)
- NATS Streaming: nats://localhost:4222 (cluster id: test-cluster)
- Kafka: localhost:9092
- Postgres: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
- MySQL: githooks:githooks@tcp(localhost:3306)/githooks

## Testing with a Local Publisher
Start the server:
```bash
export GITHUB_WEBHOOK_SECRET=devsecret
go run main.go
```

Send a test webhook (pull request opened):
```bash
curl -X POST http://localhost:8080/webhooks/github \
  -H "X-GitHub-Event: pull_request" \
  -H "X-Hub-Signature-256: sha256=$(printf '{}' | openssl dgst -sha256 -hmac devsecret | sed 's/^.* //')" \
  -H "Content-Type: application/json" \
  -d '{"action":"opened","pull_request":{"draft":false,"merged":false,"base":{"ref":"main"},"head":{"ref":"feature"}}}'
```

## Minimal Code Test (Custom Driver)
This registers a custom driver on top of gochannel and publishes a single event:
```go
internal.RegisterPublisherDriver("gochannel-custom", func(cfg internal.WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
	pub := gochannel.NewGoChannel(
		gochannel.Config{OutputChannelBuffer: 1},
		logger,
	)
	return pub, nil, nil
})
```

## Configuration
`app.yaml`
```yaml
server:
  port: 8080

providers:
  github:
    enabled: true
    path: /webhooks/github
    secret: ${GITHUB_WEBHOOK_SECRET}

watermill:
  driver: gochannel
```

`config.yaml`
```yaml
rules:
  - when: action == "opened" && draft == false
    emit: pr.opened.ready
  - when: action == "closed" && merged == true
    emit: pr.merged
```

## Normalized Event Model
Provider: github, gitlab, bitbucket
Name:     pull_request, push, ...
Data:     flattened payload fields used by rules

## Watermill Drivers
gochannel:
```yaml
watermill:
  driver: gochannel
  gochannel:
    output_buffer: 64
    persistent: false
    block_publish_until_subscriber_ack: false
```

kafka:
```yaml
watermill:
  driver: kafka
  kafka:
    brokers: ["localhost:9092"]
```

nats (streaming):
```yaml
watermill:
  driver: nats
  nats:
    cluster_id: test-cluster
    client_id: githooks
    url: nats://localhost:4222
```

amqp:
```yaml
watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
```

sql:
```yaml
watermill:
  driver: sql
  sql:
    driver: postgres
    dsn: postgres://user:pass@localhost:5432/dbname?sslmode=disable
    dialect: postgres
    auto_initialize_schema: true
```

http:
```yaml
watermill:
  driver: http
  http:
    mode: topic_url
```

## Notes
- SQL publishing requires a database driver import (e.g., lib/pq or go-sql-driver/mysql) in your app.
- Rules are evaluated in order; multiple matches publish multiple topics.
- Custom Watermill drivers can be registered at runtime via `internal.RegisterPublisherDriver`.

Example custom driver (wraps gochannel with custom config):
```go
internal.RegisterPublisherDriver("gochannel-custom", func(cfg internal.WatermillConfig, logger watermill.LoggerAdapter) (message.Publisher, func() error, error) {
	pub := gochannel.NewGoChannel(
		gochannel.Config{
			OutputChannelBuffer: 256,
			Persistent:          true,
		},
		logger,
	)
	return pub, nil, nil
})
```

## Testing
```bash
go test ./...
```

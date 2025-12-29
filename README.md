# Githooks

Config-driven webhook router for GitHub (with GitLab/Bitbucket planned). It normalizes inbound webhook events, evaluates them against YAML rules, and publishes matching events to Watermill topics for downstream consumers. This allows for flexible and dynamic event-driven architectures based on your Git activity.

## Features
- Typed webhook parsing via go-playground/webhooks
- All GitHub webhook events supported
- Provider-agnostic normalized event model
- Rule-based routing via govaluate
- Watermill-backed publishing (gochannel, Kafka, NATS Streaming, AMQP, SQL, HTTP)
- Stateless by default

## Quickstart

### Local Development with Docker Compose
One command builds and runs the agent on port 8080 with all local drivers enabled via `config.yaml` (which is derived from `app.docker.yaml`):
```bash
docker-compose up --build
```

You can override the default webhook secret if needed:
```bash
GITHUB_WEBHOOK_SECRET=yoursecret docker-compose up --build
```

Then send GitHub webhooks to:
`http://localhost:8080/webhooks/github`

**Useful endpoints provided by Docker Compose:**
- RabbitMQ UI: http://localhost:15672 (guest/guest)
- NATS Streaming: nats://localhost:4222 (cluster id: test-cluster)
- Kafka: localhost:9092
- Postgres: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
- MySQL: githooks:githooks@tcp(localhost:3306)/githooks

### Testing with a Local Publisher
Send a test webhook (pull request opened event) to your local Githooks instance:
```bash
body='{"action":"opened","pull_request":{"draft":false,"merged":false,"base":{"ref":"main"},"head":{"ref":"feature"}}}'
sig=$(printf '%s' "$body" | openssl dgst -sha1 -hmac devsecret | sed 's/^.* //')
curl -X POST http://localhost:8080/webhooks/github \
  -H "X-GitHub-Event: pull_request" \
  -H "X-Hub-Signature: sha1=$sig" \
  -H "Content-Type: application/json" \
  -d "$body"
```

### Production Setup (GitHub App)
For production use, especially with GitHub, consider using a GitHub App:
1.  Create a GitHub App in your organization/user settings.
2.  Set the webhook URL to `https://<your-domain>/webhooks/github`.
3.  Set the webhook secret and expose it as an environment variable (e.g., `GITHUB_WEBHOOK_SECRET`).
4.  Subscribe to the specific GitHub events you need (e.g., pull requests, pushes).
5.  Deploy the Githooks service, ensuring the endpoint is publicly reachable.

## Configuration

Githooks is configured using a `config.yaml` file. The Docker Compose setup uses `app.docker.yaml` as an example which gets mapped to `config.yaml` inside the container.

### Provider Configuration
Providers define how Githooks receives webhook events.

**GitHub (enabled by default):**
```yaml
providers:
  github:
    enabled: true
    path: /webhooks/github
    secret: ${GITHUB_WEBHOOK_SECRET} # Use environment variable for secret
```

**GitLab / Bitbucket (planned):**
```yaml
providers:
  gitlab:
    enabled: false
  bitbucket:
    enabled: false
```

### Supported Providers
| Provider | Status |
| --- | --- |
| GitHub | Supported |
| GitLab | Planned |
| Bitbucket | Planned |


### Watermill Drivers (Publishers)
Watermill drivers are used to publish normalized events to various messaging systems. You can configure a single driver or multiple drivers for fan-out.

**Example `config.yaml` (GitHub + AMQP):**
```yaml
server:
  port: 8080

providers:
  github:
    enabled: true
    path: /webhooks/github
    secret: ${GITHUB_WEBHOOK_SECRET}

watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
```

**Available Driver Configurations:**

**GoChannel:**
```yaml
watermill:
  driver: gochannel
  gochannel:
    output_buffer: 64
    persistent: false
    block_publish_until_subscriber_ack: false
```

**Kafka:**
```yaml
watermill:
  driver: kafka
  kafka:
    brokers: ["localhost:9092"]
```

**NATS Streaming:**
```yaml
watermill:
  driver: nats
  nats:
    cluster_id: test-cluster
    client_id: githooks
    url: nats://localhost:4222
```

**AMQP (RabbitMQ):**
```yaml
watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
```

**SQL (Postgres/MySQL):**
```yaml
watermill:
  driver: sql
  sql:
    driver: postgres # or mysql
    dsn: postgres://user:pass@localhost:5432/dbname?sslmode=disable
    dialect: postgres # or mysql
    auto_initialize_schema: true
```

**HTTP:**
```yaml
watermill:
  driver: http
  http:
    mode: topic_url # or base_url for fan-out (see below)
```

**Multiple Drivers (Fan-out):**
To publish an event to multiple messaging systems, specify `drivers` and configure each one:
```yaml
watermill:
  drivers: [amqp, http]
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
  http:
    mode: base_url
    base_url: http://localhost:9000/hooks
```

### Rule Engine
Githooks uses a rule engine to filter and route normalized webhook events. Rules are defined in your `config.yaml` file.

**Normalized Event Model:**
Webhook events are normalized into a common structure:
- `Provider`: e.g., `github`, `gitlab`, `bitbucket`
- `Name`: e.g., `pull_request`, `push`, `issue_comment`
- `Data`: A flattened payload of fields from the original webhook, used for rule evaluation.

**Rule Syntax:**
Rules use JSONPath for field access and `govaluate` for boolean logic.

```yaml
rules:
  - when: $.action == "opened" && $.pull_request.draft == false
    emit: pr.opened.ready
  - when: $.action == "closed" && $.pull_request.merged == true
    emit: pr.merged
    drivers: [amqp, http] # Target specific drivers for this rule
```

-   **JSONPath examples**:
    -   `$.pull_request.draft == false`
    -   `$.pull_request.commits[0].created == true`
    -   `$.pull_request[0].draft == false`
-   Bare identifiers are treated as root JSONPath (e.g., `action` becomes `$.action`).
-   Flattened data is still included in the emitted event: `pull_request.draft`, `pull_request.commits[0].created`, `pull_request.commits[]` (full array).

**Rule Driver Targeting:**
-   The `drivers` field is optional on each rule.
-   When `drivers` is omitted, the event is published to all configured Watermill drivers.
-   When `drivers` is specified, the event is only published to those named drivers.

**Rules Strict Mode:**
-   Set `rules_strict: true` in your `config.yaml` to skip rule evaluation if any JSONPath expression within a rule's `when` clause is missing from the event data.

## Architecture
```
Webhook Provider -> go-playground/webhooks -> Adapter -> Normalized Event -> Rule Engine -> Watermill Publisher
```

## Development

### Running Tests
To run all unit and integration tests:
```bash
go test ./...
```

### Custom Watermill Drivers
You can register custom Watermill drivers at runtime via `internal.RegisterPublisherDriver`. This allows for extending Githooks with support for new messaging systems or custom publishing logic.

**Example Custom Driver (wraps GoChannel):**
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

## Notes
-   SQL publishing requires importing the appropriate database driver (e.g., `github.com/lib/pq` for Postgres or `github.com/go-sql-driver/mysql` for MySQL) in your application's `main.go` file.
-   Rules are evaluated in order; if multiple rules match an event, multiple topics will be published.
-   The default webhook secret for testing is `devsecret`.

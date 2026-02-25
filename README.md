# githook ⚡

> **⚠️ Warning:** Research and development only. Not production-ready.

githook is a webhook router for GitHub/GitLab/Bitbucket. It receives events, evaluates rules, and publishes matching events to AMQP/NATS/Kafka. Workers consume events and can fetch SCM clients (GitHub/GitLab/Bitbucket) from the server.

## What it includes ✨

- Unified webhook endpoints for GitHub, GitLab, Bitbucket
- Rule-based routing (`when` + `emit`) with stored rules
- Driver configs stored per tenant (AMQP/NATS/Kafka)
- Worker SDKs (Go/TypeScript/Python) using rule IDs
- Optional OAuth2-authenticated API
- Event log storage with headers/body + body hash

## Install 🧰

```bash
brew install relaymesh/homebrew-formula/githook
```

Or from source:

```bash
go build -o githook ./main.go
```

## Quick start (local) 🚀

1) Start the server:

```bash
githook serve --config config.yaml
```

Minimal `config.yaml`:

```yaml
server:
  port: 8080

endpoint: http://localhost:8080

storage:
  driver: postgres
  dsn: postgres://githook:githook@localhost:5432/githook?sslmode=disable
  dialect: postgres
  auto_migrate: true
```

2) Register a provider instance (YAML):

```bash
githook --endpoint http://localhost:8080 providers create \
  --provider github \
  --config-file github.yaml
```

Example `github.yaml`:

```yaml
app:
  app_id: 12345
  private_key_path: ./github.pem
oauth:
  client_id: your-client-id
  client_secret: your-client-secret
webhook:
  secret: your-webhook-secret
```

3) Create a driver config (YAML):

```bash
githook --endpoint http://localhost:8080 drivers create \
  --name amqp \
  --config-file amqp.yaml
```

Example `amqp.yaml`:

```yaml
url: amqp://guest:guest@localhost:5672/
exchange: githook.events
routing_key_template: "{topic}"
```

Make sure the exchange exists in your broker.

4) Create a rule:

```bash
githook --endpoint http://localhost:8080 rules create \
  --when 'action == "opened"' \
  --emit pr.opened \
  --driver-id <driver-id>
```

5) Point your provider webhook to:

```
http://<server-host>/webhooks/github
http://<server-host>/webhooks/gitlab
http://<server-host>/webhooks/bitbucket
```

## CLI essentials 🧭

- Providers: `providers list|get|create|update|delete`
- Drivers: `drivers list|get|create|update|delete`
- Rules: `rules list|get|create|update|delete|match`
- Namespaces: `namespaces list|update` and `namespaces webhook get|update`
- Installations: `installations list|get`

## Worker SDKs (rule id) 🛠️

Go:

```go
wk := worker.New(worker.WithEndpoint("http://localhost:8080"))

wk.HandleRule("<rule-id>", func(ctx context.Context, evt *worker.Event) error {
	if evt == nil {
		return nil
	}
	log.Printf("topic=%s provider=%s type=%s", evt.Topic, evt.Provider, evt.Type)
	return nil
})

_ = wk.Run(ctx)
```

TypeScript:

```ts
import { New, WithEndpoint } from "@relaymesh/githook";

const worker = New(WithEndpoint("http://localhost:8080"));

worker.HandleRule("<rule-id>", async (evt) => {
  console.log(evt.topic, evt.provider, evt.type);
});

await worker.Run();
```

Python:

```python
from relaymesh_githook import New, WithEndpoint

wk = New(WithEndpoint("http://localhost:8080"))

wk.HandleRule("<rule-id>", lambda ctx, evt: print(evt.topic, evt.provider, evt.type))

wk.Run()
```

## Docs 📚

See `docs/` for provider setup, OAuth flows, rules, drivers, event logs, and SDK details.

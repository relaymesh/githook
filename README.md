# githook ⚡

> **⚠️ Warning:** Research and development only. Not production-ready.

githook is a multi-tenant webhook router for GitHub, GitLab, and Bitbucket. It receives webhook events, evaluates rules, and publishes matching events to AMQP, NATS, or Kafka. Workers subscribe to those topics and can request SCM clients from the server.

## Core concepts

- Provider instance: a per-tenant provider configuration (OAuth + webhook secret + optional enterprise URLs).
- Driver: a broker configuration (AMQP/NATS/Kafka) stored per tenant.
- Rule: a `when` expression plus `emit` topic(s) targeting a driver.
- Worker: consumes published events and can request SCM clients from the server.
- Tenant: logical workspace selected by `X-Tenant-ID` or `--tenant-id`.
- Event log: stored webhook headers/body plus a body hash for auditing.

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

4) Create a rule:

```bash
githook --endpoint http://localhost:8080 rules create \
  --when 'action == "opened"' \
  --emit pr.opened \
  --driver-id <driver-id>
```

`--driver-id` is the driver record ID (see `githook drivers list`).

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
var attempts atomic.Uint64

wk := worker.New(worker.WithEndpoint("http://localhost:8080"))

wk.HandleRule("<rule-id>", func(ctx context.Context, evt *worker.Event) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("panic recovered: %v", recovered)
		}
	}()

	if evt == nil {
		return nil
	}

	seq := attempts.Add(1)
	if seq%2 == 0 {
		return fmt.Errorf("intentional failure for status test (seq=%d)", seq)
	}

	log.Printf("success seq=%d topic=%s provider=%s type=%s", seq, evt.Topic, evt.Provider, evt.Type)
	return nil
})

_ = wk.Run(ctx)
```

TypeScript:

```ts
import { New, WithEndpoint } from "@relaymesh/githook";

const worker = New(WithEndpoint("http://localhost:8080"));
let attempts = 0;

worker.HandleRule("<rule-id>", async (evt) => {
  attempts += 1;
  try {
    if (attempts % 2 === 0) {
      throw new Error(`intentional failure for status test (seq=${attempts})`);
    }

    console.log("success", attempts, evt.topic, evt.provider, evt.type);
  } catch (err) {
    const error = err instanceof Error ? err : new Error(String(err));
    console.error("handler failed", attempts, error.message);
    throw error;
  }
});

await worker.Run();
```

Python:

```python
from relaymesh_githook import New, WithEndpoint

wk = New(WithEndpoint("http://localhost:8080"))
attempts = 0

def handler(ctx, evt):
    global attempts
    attempts += 1
    try:
        if attempts % 2 == 0:
            raise RuntimeError(f"intentional failure for status test (seq={attempts})")

        print("success", attempts, evt.topic, evt.provider, evt.type)
    except Exception as exc:
        print("handler failed", attempts, str(exc))
        raise

wk.HandleRule("<rule-id>", handler)

wk.Run()
```

For these examples, every second event fails on purpose so you can validate event log status transitions (`success` vs `failed`) end to end.

## Docs index 📚

- Getting started: `docs/getting-started-github.md`, `docs/getting-started-gitlab.md`, `docs/getting-started-bitbucket.md`
- CLI: `docs/cli.md`
- Rules: `docs/rules.md`
- Drivers: `docs/drivers.md`
- Auth: `docs/auth.md`
- Events: `docs/events.md`
- Observability: `docs/observability.md`
- SDK clients: `docs/sdk_clients.md`

# Githooks ⚡

Githooks is an event automation layer for GitHub, GitLab, and Bitbucket. It receives webhooks, evaluates configurable rules, and publishes matching events to your message broker via [Watermill](https://watermill.io/). The Worker SDK then consumes those events with provider-aware clients, so your business logic stays focused on outcomes, not plumbing.

> **⚠️ Warning:** This project is for research and development only and is **not production-ready**. Do not deploy it in production environments.

## Quick Start

Get Githooks running locally in 4 steps:

```bash
# 1. Start dependencies (RabbitMQ, Postgres, etc.)
docker compose up -d

# 2. Configure GitHub webhook secret
export GITHUB_WEBHOOK_SECRET=devsecret

# 3. Start the server
go run ./main.go serve --config config.yaml

# 4. In another terminal, start a worker
go run ./example/github/worker/main.go --config config.yaml --driver amqp
```

Now send a test webhook:
```bash
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

**Next steps:** See [Getting Started (Local)](#getting-started-local) for detailed setup, or jump to provider-specific guides:
- [GitHub Setup](docs/getting-started-github.md)
- [GitLab Setup](docs/getting-started-gitlab.md)
- [Bitbucket Setup](docs/getting-started-bitbucket.md)

## Prerequisites

- **Go 1.21+**
- **Docker + Docker Compose** (for local development)
- **PostgreSQL** (for installation storage)
- **Message Broker**: RabbitMQ, NATS, Kafka, or any [Watermill-supported broker](https://watermill.io/docs/pub-subs/)
- **Provider Credentials**:
  - GitHub: App ID, Private Key, OAuth Client (optional)
  - GitLab: OAuth Application credentials
  - Bitbucket: OAuth Consumer credentials

## How It Works

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐      ┌─────────────┐
│   GitHub    │      │              │      │   Message   │      │   Workers   │
│   GitLab    │─────▶│  Githooks    │─────▶│   Broker    │─────▶│  (Your App) │
│  Bitbucket  │      │   Server     │      │   (AMQP)    │      │             │
└─────────────┘      └──────────────┘      └─────────────┘      └─────────────┘
   Webhooks           Rules Engine          Watermill           Business Logic
                      + Publishing                              + SCM Clients
```

1. **Receive**: Githooks receives webhooks from GitHub, GitLab, or Bitbucket
2. **Evaluate**: Rules engine evaluates JSONPath conditions against the payload
3. **Publish**: Matching events are published to your message broker
4. **Consume**: Workers consume events with provider-aware SCM clients ready to use

## Why Githooks ✨

- **Unify SCM events** without writing three webhook stacks. 🔗
- **Route events by rules** (JSONPath + boolean logic) instead of hardcoding. 🧠
- **Use any broker** supported by Watermill, with optional fan-out per rule. 📬
- **Act with real clients** (GitHub App, GitLab, Bitbucket) inside workers. 🔐
- **Multi-tenant ready** with provider instance management and OAuth onboarding. 🏢

## Features ✅

- **Multi-Provider Webhooks**: GitHub, GitLab, and Bitbucket. 🌍
- **Rule Engine**: JSONPath + boolean rules with multi-match support. 🧩
- **Protobuf Event Envelope**: Broker payloads use `cloud.v1.EventPayload`, with raw JSON preserved. 📦
- **Flexible Publishing**: AMQP, NATS Streaming, Kafka, HTTP, SQL, GoChannel, RiverQueue. 🚚
- **Multi-Driver Fan-Out**: Publish to all drivers by default or target per rule. 🧯
- **Worker SDK**: Concurrency, middleware, topics, and graceful shutdown. 🧰
- **SCM Auth Resolution**: GitHub App (JWT → installation token), GitLab/Bitbucket OAuth tokens stored on install. 🔑
- **Observability**: Request IDs and structured logs. 🔎
- **Ship-Ready Assets**: Docker Compose, examples, boilerplate, Helm charts. 📚

## Common Use Cases 🚀

- **Release orchestration**: Trigger CI/CD or internal workflows from PR merges.
- **Preview automation**: Post preview links on PR/MR events across providers.
- **Compliance hooks**: Enforce policy when branch protection or approvals change.

## Table of Contents

- [Quick Start](#quick-start)
- [Prerequisites](#prerequisites)
- [How It Works](#how-it-works)
- [Why Githooks](#why-githooks-)
- [Features](#features-)
- [Common Use Cases](#common-use-cases-)
- [Getting Started (Local)](#getting-started-local)
- [OAuth Onboarding](#oauth-onboarding)
- [Configuration](#configuration)
  - [Providers](#providers)
  - [Server Settings](#server-limits)
  - [OAuth Callbacks](#oauth-callbacks)
  - [Storage](#installation-storage)
  - [Watermill Drivers](#watermill-drivers-publishing)
  - [Rules Engine](#rules)
- [Worker SDK](#worker-sdk)
- [CLI Commands](#api-endpoints-connectgrpc)
- [Examples](#examples)
- [Helm Charts](#helm-charts)
- [Troubleshooting](#troubleshooting)
- [Development](#development)
- [Releases](#releases)

## Getting Started (Local)

1.  **Start dependencies:**

    ```bash
    docker compose up -d
    ```

2.  **Run the server:**

    Set the secret for validating GitHub webhooks and run the server with the local Docker config.

    ```bash
    export GITHUB_WEBHOOK_SECRET=devsecret
    go run ./main.go serve --config app.docker.yaml
    ```

3.  **Run a worker:**

    In another terminal, run an example worker that listens for events.

    ```bash
    go run ./example/github/worker/main.go --config app.docker.yaml
    ```

4.  **Send a test webhook:**

    Use the provided script to simulate a GitHub `pull_request` event.

    ```bash
    ./scripts/send_webhook.sh github pull_request example/github/pull_request.json
    ```

    You should see the server log the event and the worker log its "pr.opened.ready" message.

**Single‑binary (in‑process)**

Use GoChannel to run the server and multiple workers in one process:

```bash
go run ./example/inprocess/main.go --config app.docker.yaml
```

## OAuth Onboarding

Githooks supports OAuth-based onboarding for GitLab and Bitbucket, and GitHub Apps with user authorization.

### Start Onboarding Flow

Redirect users to initiate the OAuth flow:

```
http://localhost:8080/?provider=github&instance=<instance-hash>
http://localhost:8080/?provider=gitlab&instance=<instance-hash>
http://localhost:8080/?provider=bitbucket&instance=<instance-hash>
```

### Multiple Provider Instances

If you have multiple provider instances configured (e.g., GitHub.com + GitHub Enterprise), specify which instance:

```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

**Get instance hash using CLI:**
```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider github
```

This returns all configured provider instances with their server-generated instance hashes.

### OAuth Callback URLs

When configuring OAuth applications, use these callback paths:

- **GitHub**: `https://your-domain.com/auth/github/callback`
- **GitLab**: `https://your-domain.com/auth/gitlab/callback`
- **Bitbucket**: `https://your-domain.com/auth/bitbucket/callback`

**Important:** The path must be `/auth/{provider}/callback` (not `/oauth/{provider}/callback`).

For local development with ngrok:
```yaml
server:
  public_base_url: https://your-ngrok-url.ngrok-free.app
```

See provider-specific guides for detailed OAuth setup:
- [GitHub OAuth Setup](docs/getting-started-github.md#step-5-create-a-github-app)
- [GitLab OAuth Setup](docs/getting-started-gitlab.md#6-create-a-gitlab-oauth-application)
- [Bitbucket OAuth Setup](docs/getting-started-bitbucket.md#6-configure-a-bitbucket-webhook)

## Configuration

Docs:
- [Driver configuration](docs/drivers.md)
- [Event compatibility](docs/events.md)
- [Getting started (GitHub)](docs/getting-started-github.md)
- [Getting started (GitLab)](docs/getting-started-gitlab.md)
- [Getting started (Bitbucket)](docs/getting-started-bitbucket.md)
- [Rules engine](docs/rules.md)
- [Observability](docs/observability.md)
- [SCM authentication](docs/scm-auth.md)
- [Installation storage](docs/storage.md)
- [CLI usage](docs/cli.md)
- [OAuth callbacks](docs/oauth-callbacks.md)
- [Webhook setup](docs/webhooks.md)
- [SDK client injection](docs/sdk_clients.md)
- [SDK DSL (portable worker spec)](docs/sdk-dsl.md)
- [API authentication (OAuth2/OIDC)](docs/auth.md)
- [Secure API quickstart](docs/quickstart-secure.md)

Githooks is configured using a single YAML file. Environment variables like `${VAR}` are automatically expanded.
Requests use or generate `X-Request-Id`, which is echoed back in responses and included in logs.

### Providers

The `providers` section configures webhook endpoints and SCM auth for each Git provider.
Provider instances created from config are stored on startup with a server-generated instance hash. Use the Providers API to fetch it when you need to target a specific instance.
If `webhook.path` is omitted, defaults are used: `/webhooks/github`, `/webhooks/gitlab`, `/webhooks/bitbucket`.
Set `server.public_base_url` when running behind ngrok or a reverse proxy so OAuth callbacks resolve to your public domain.
`providers.*.oauth` is reserved for OAuth2 expansion in future releases.

```yaml
providers:
  github:
    enabled: true
    webhook:
      path: /webhooks/github
      secret: ${GITHUB_WEBHOOK_SECRET}
    app:
      app_id: ${GITHUB_APP_ID}
      private_key_path: ${GITHUB_PRIVATE_KEY_PATH}
      app_slug: ${GITHUB_APP_SLUG}
    api:
      base_url: https://api.github.com
      web_base_url: https://github.com
    oauth:
      client_id: ${GITHUB_OAUTH_CLIENT_ID}
      client_secret: ${GITHUB_OAUTH_CLIENT_SECRET}
      scopes: ["read:user"]
  gitlab:
    enabled: false
    webhook:
      path: /webhooks/gitlab
      secret: ${GITLAB_WEBHOOK_SECRET} # Optional
    api:
      base_url: https://gitlab.com/api/v4
      web_base_url: https://gitlab.com
    oauth:
      client_id: ${GITLAB_OAUTH_CLIENT_ID}
      client_secret: ${GITLAB_OAUTH_CLIENT_SECRET}
      scopes: ["read_api"]
  bitbucket:
    enabled: false
    webhook:
      path: /webhooks/bitbucket
      secret: ${BITBUCKET_WEBHOOK_SECRET} # Optional, for X-Hook-UUID
    api:
      base_url: https://api.bitbucket.org/2.0
      web_base_url: https://bitbucket.org
    oauth:
      client_id: ${BITBUCKET_OAUTH_CLIENT_ID}
      client_secret: ${BITBUCKET_OAUTH_CLIENT_SECRET}
      scopes: ["repository"]
```

### SCM Authentication

```yaml
providers:
  github:
    app:
      app_id: 123
      private_key_path: /secrets/github.pem
      app_slug: githooks
    api:
      base_url: https://api.github.com
      web_base_url: https://github.com
  gitlab:
    api:
      base_url: https://gitlab.com/api/v4
      web_base_url: https://gitlab.com
  bitbucket:
    api:
      base_url: https://api.bitbucket.org/2.0
      web_base_url: https://bitbucket.org
```

GitHub Enterprise: set `providers.github.api.base_url` to your API base (for example,
`https://ghe.example.com/api/v3`). The SDK derives the upload URL automatically.

### Server Limits

```yaml
server:
  port: 8080
  public_base_url: https://app.example.com
  read_timeout_ms: 5000
  write_timeout_ms: 10000
  idle_timeout_ms: 60000
  read_header_timeout_ms: 5000
  max_body_bytes: 1048576
  debug_events: false
```

### API Authentication

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
```

When enabled, all Connect RPC endpoints require a bearer token. Webhooks and `/auth/*` remain public.
See `docs/auth.md` for client_credentials and human login flows.

### Installation Storage

```yaml
storage:
  driver: postgres
  dsn: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
  dialect: postgres
  auto_migrate: true
```

### OAuth Callbacks

Configure where users are redirected after completing OAuth authorization:

```yaml
oauth:
  redirect_base_url: https://app.example.com/oauth/complete
```

**OAuth Callback Endpoints** (configured in provider settings):
- GitHub: `/auth/github/callback` ✅ (not `/oauth/github/callback` ❌)
- GitLab: `/auth/gitlab/callback` ✅
- Bitbucket: `/auth/bitbucket/callback` ✅

**Notes:**
- GitHub App installs are initiated from the GitHub App installation page
- The GitHub callback is only used when "Request user authorization (OAuth)" is enabled
- Always set `server.public_base_url` when running behind ngrok or a reverse proxy
- The callback receives authorization codes and exchanges them for access tokens
- See [OAuth Onboarding](#oauth-onboarding) for detailed setup instructions

### API Endpoints (Connect/GRPC)

REST endpoints are replaced by Connect/GRPC handlers. Use the generated client from
`pkg/gen/cloud/v1/cloudv1connect` or call the procedures directly.

```text
/cloud.v1.InstallationsService/ListInstallations
/cloud.v1.InstallationsService/GetInstallationByID
/cloud.v1.NamespacesService/ListNamespaces
/cloud.v1.NamespacesService/SyncNamespaces
/cloud.v1.NamespacesService/GetNamespaceWebhook
/cloud.v1.NamespacesService/SetNamespaceWebhook
/cloud.v1.RulesService/MatchRules
/cloud.v1.DriversService/ListDrivers
/cloud.v1.DriversService/GetDriver
/cloud.v1.DriversService/UpsertDriver
/cloud.v1.DriversService/DeleteDriver
/cloud.v1.ProvidersService/ListProviders
/cloud.v1.ProvidersService/GetProvider
/cloud.v1.ProvidersService/UpsertProvider
/cloud.v1.ProvidersService/DeleteProvider
```

Use the Connect RPC to get the provider URL and state.

Notes:
- GitHub webhooks are always enabled by the GitHub App and cannot be toggled.
- GitLab/Bitbucket create/delete provider webhooks when toggled.

CLI shortcuts (via Connect RPC):
```bash
githooks --endpoint http://localhost:8080 installations list --state-id <state-id>
githooks --endpoint http://localhost:8080 installations get --provider github --installation-id <id>
githooks --endpoint http://localhost:8080 namespaces list --state-id <state-id>
githooks --endpoint http://localhost:8080 namespaces sync --state-id <state-id> --provider gitlab
githooks --endpoint http://localhost:8080 namespaces webhook get --state-id <state-id> --provider gitlab --repo-id <repo-id>
githooks --endpoint http://localhost:8080 namespaces webhook set --state-id <state-id> --provider gitlab --repo-id <repo-id> --enabled
githooks --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml
githooks --endpoint http://localhost:8080 providers list --provider github
githooks --endpoint http://localhost:8080 providers get --provider github --hash <instance-hash>
githooks --endpoint http://localhost:8080 providers set --provider github --config-file github.json
githooks --endpoint http://localhost:8080 providers delete --provider github --hash <instance-hash>
githooks --endpoint http://localhost:8080 drivers list
githooks --endpoint http://localhost:8080 drivers get --name amqp
githooks --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.json
githooks --endpoint http://localhost:8080 drivers delete --name amqp
```

### Install/Authorize Entry

Start an install/authorize flow by redirecting users to:

```
http://localhost:8080/?provider=github
http://localhost:8080/?provider=gitlab
http://localhost:8080/?provider=bitbucket
```

**With Multiple Provider Instances:**

When you have multiple provider instances configured (e.g., GitHub.com and GitHub Enterprise), you must specify which instance using the instance hash:

```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

Get the instance hash using the CLI:
```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider github
```

**How It Works:**
- **GitHub**: Redirects to GitHub App installation page (`https://github.com/apps/{app_slug}/installations/new`)
- **GitLab**: Redirects to OAuth authorize URL (`https://gitlab.com/oauth/authorize`)
- **Bitbucket**: Redirects to OAuth authorize URL (`https://bitbucket.org/site/oauth2/authorize`)

**Optional Parameters:**
- `tenant_id`: Multi-tenant identifier for scoping installations
- `state`: Custom CSRF state value (auto-generated if not provided)

See [OAuth Onboarding](#oauth-onboarding) for detailed setup instructions.

### Watermill Drivers (Publishing)

The `watermill` section configures the message broker(s) to publish events to.

-   `driver`: (string) Default publisher driver.
-   `drivers`: (array) Fan-out to all listed drivers by default.

**Single Driver (AMQP)**
```yaml
watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue # Or: nondurable_queue, durable_pubsub, nondurable_pubsub
```


**Multiple Drivers (Fan-Out)**
```yaml
watermill:
  drivers: [amqp, http]
  amqp:
    url: amqp://guest:guest@localhost:5672/
  http:
    mode: base_url
    base_url: http://localhost:9000/hooks
```

**RiverQueue (Postgres Job Queue)**
```yaml
watermill:
  driver: riverqueue
  riverqueue:
    driver: postgres
    dsn: postgres://user:pass@localhost:5432/dbname?sslmode=disable
    table: river_job # Optional, default is river_job
    queue: default   # Optional, default is default
    kind: githooks.event # The job type to insert
```

See the [Watermill documentation](https://watermill.io/docs/pub-subs/) for details on each driver's configuration.

### Rules

The `rules` section defines which events to publish and where. Each rule has a `when` condition and an `emit` topic.

```yaml
rules_strict: false # Optional: if true, rules with missing fields won't match
rules:
  # If a PR is opened and not a draft, emit to 'pr.opened.ready'
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready

  # If a PR is merged, emit to 'pr.merged' on specific drivers
  - when: action == "closed" && pull_request.merged == true
    emit: pr.merged
    drivers: [amqp, http]

  # Fan-out to multiple topics
  - when: action == "closed" && pull_request.merged == true
    emit: [pr.merged, audit.pr.merged]
```

-   **`when`**: A boolean expression evaluated against the webhook payload.
    -   Bare identifiers (e.g., `action`) are treated as JSONPath `$.action`.
    -   You can use full JSONPath syntax (e.g., `$.pull_request.head.ref`).
    -   Helper functions: `contains(value, needle)` and `like(value, pattern)` (`%` wildcard).
-   **`emit`**: The topic name to publish the event to if the `when` condition is true.
-   **`emit`** can also be a list to publish to multiple topics.
-   **`drivers`**: (Optional) A list of specific drivers to publish this event to. If omitted, the default `driver` or `drivers` from the Watermill config are used.

## Worker SDK

The worker SDK provides a simple way to consume events from the message broker.

**Minimal Example**
```go
package main

import (
    "context"
    "log"

    "githooks/sdk/go/worker"
)

func main() {
    // Load subscriber settings from the same config file the server uses.
    subCfg, err := worker.LoadSubscriberConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to build subscriber: %v", err)
    }
    sub, err := worker.BuildSubscriber(subCfg)
    if err != nil {
        log.Fatalf("Failed to build subscriber: %v", err)
    }

    wk := worker.New(
        worker.WithSubscriber(sub),
        worker.WithTopics("pr.opened.ready"), // List of topics to subscribe to
        worker.WithConcurrency(10),
    )

    // Register a handler for a specific topic
    wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
        log.Printf("Received event: %s/%s", evt.Provider, evt.Type)
        // Do something with evt.Payload or evt.Normalized
        return nil
    })

    // Run the worker (blocking call)
    if err := wk.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

If you need provider‑aware clients inside handlers, set `server.public_base_url` in your worker config
or export `GITHOOKS_API_BASE_URL` so the worker can resolve installation tokens.

**Watermill Middleware**

You can use any Watermill middleware with the provided adapter.

```go
import wmmw "github.com/ThreeDotsLabs/watermill/message/router/middleware"

retryMiddleware := worker.MiddlewareFromWatermill(
    wmmw.Retry{MaxRetries: 3}.Middleware,
)

wk := worker.New(
  // ... other options
  worker.WithMiddleware(retryMiddleware),
)
```

## Building Your Own Go App

If you like this model of Git provider webhook management, you can build your own Go app by reusing the same pattern: validate provider signatures, normalize payloads, evaluate rules, then publish to a broker and consume with workers. Use the SDK to wire provider clients into handlers and keep business logic isolated from transport.

## Examples

The `example/` directory contains several working examples:
-   **`example/github`**: A simple server and worker for handling GitHub webhooks.
-   **`example/realworld`**: A more complex setup with multiple workers consuming events from a single topic.
-   **`example/riverqueue`**: Demonstrates publishing events to a [River](https://github.com/riverqueue/river) job queue.
-   **`example/vercel`**: Production-style preview/production deploy hooks for Vercel.
-   **`example/gitlab`**: Sample setup for GitLab webhooks.
-   **`example/bitbucket`**: Sample setup for Bitbucket webhooks.
-   **`example/inprocess`**: Single-binary server + multiple workers using GoChannel.

## Helm Charts

Helm charts for deploying the server and a generic worker are available in the `charts/` directory.

**Install from GitHub Pages**
```sh
helm repo add githooks https://yindia.github.io/githooks
helm repo update
helm install my-githooks githooks/githooks
helm install my-worker githooks/githooks-worker
```

## Troubleshooting

### Common Issues

#### OAuth Callback 404 Error
**Error:** `404 page not found` when redirected after OAuth authorization

**Solution:** Verify your OAuth application callback URL uses the correct path:
- ✅ Correct: `/auth/github/callback`, `/auth/gitlab/callback`, `/auth/bitbucket/callback`
- ❌ Incorrect: `/oauth/github/callback`

#### Database Constraint Error
**Error:** `ERROR: there is no unique or exclusion constraint matching the ON CONFLICT specification`

**Solution:** The database schema needs to be recreated with the proper unique constraint:
```bash
# Drop the table
docker exec -i githooks-postgres-1 psql -U githooks -d githooks -c "DROP TABLE IF EXISTS githooks_installations CASCADE;"

# Restart the server to recreate the table with correct schema
go run ./main.go serve --config config.yaml
```

#### Duplicate Installation Entries
**Problem:** Multiple installation records for the same provider/installation

**Solution:** This was a bug in versions prior to the fix. Clean up duplicates:
```bash
# Delete entries with empty account names (invalid entries)
docker exec -i githooks-postgres-1 psql -U githooks -d githooks \
  -c "DELETE FROM githooks_installations WHERE account_name = '' OR account_name IS NULL;"
```

#### Webhook Signature Mismatch
**Error:** `missing X-Hub-Signature` or `signature mismatch`

**Solution:** Ensure your webhook secret matches between the provider settings and your config:
```yaml
providers:
  github:
    webhook:
      secret: ${GITHUB_WEBHOOK_SECRET}  # Must match GitHub App webhook secret
```

#### No Matching Rules
**Problem:** Events received but not published to broker

**Solution:** Check your rules match the payload:
```bash
# Test rules against a payload
go run ./main.go --endpoint http://localhost:8080 rules match \
  --payload-file payload.json --rules-file rules.yaml
```

Enable debug logging to see rule evaluation:
```yaml
server:
  debug_events: true
```

#### Connection Refused to Broker
**Error:** `connection refused` when publishing events

**Solution:** Ensure Docker Compose services are running:
```bash
docker compose ps
docker compose up -d
```

#### Missing Provider Instance
**Problem:** Cannot find provider instance hash

**Solution:** List all configured provider instances:
```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider github
```

Providers created from config are automatically stored with server-generated hashes on startup.

### Getting Help

- **Documentation**: Check the [docs/](docs/) directory for detailed guides
- **Issues**: Report bugs at [GitHub Issues](https://github.com/yindia/githooks/issues)
- **Examples**: See working examples in [example/](example/) directory

## Releases

-   **Code Releases**: Tagging a commit with `vX.Y.Z` triggers a workflow that publishes a new Go module version and a container image to `ghcr.io/yindia/githooks`.
-   **Chart Releases**: Tagging a commit with `chart-X.Y.Z` publishes the Helm charts to the `gh-pages` branch. Ensure you update `version` and `appVersion` in `charts/*/Chart.yaml` first.

## Development

**Run Tests**
```bash
go test ./...
```

**Notes**
-   When using the SQL publisher, you must blank-import a database driver (e.g., `_ "github.com/lib/pq"`).
-   The default webhook secret for local testing is `devsecret`.
-   Rules are evaluated in the order they appear in the config file. Multiple rules can match a single event, causing multiple messages to be published.

# githook ⚡

> **⚠️ Warning:** This project is for research and development only and is **not production-ready**. Do not deploy it in production environments.

## Table of Contents

1. [About](#about)
2. [How It Works](#how-it-works)
3. [Features](#features)
4. [Why githook](#why-githook)
5. [Installing the CLI](#installing-the-cli)
6. [Quick Start Guide (GitHub Apps)](#quick-start-guide-github-apps)
7. [OAuth Onboarding Flow](#oauth-onboarding-flow)
8. [SCM-Specific Documentation](#scm-specific-documentation)
9. [Terminology](#terminology)
10. [Storage](#storage)
11. [Webhook URLs](#webhook-urls)
12. [Drivers](#drivers)
13. [Rules](#rules)
14. [SDK](#sdk)
15. [Examples](#examples)
16. [Documentation](#documentation)

---

## About

githook is an event automation layer for GitHub, GitLab, and Bitbucket. It receives webhooks from these providers, evaluates configurable rules against the payload, and publishes matching events to your message broker using [Watermill](https://watermill.io/).

**What problem does it solve?**

Managing webhooks across multiple Git providers (GitHub, GitLab, Bitbucket) typically requires:
- Writing provider-specific webhook handlers for each platform
- Manually normalizing different payload formats
- Hardcoding event routing logic in your application
- Managing authentication for each provider's API separately

githook solves this by providing:
- A unified webhook receiver for all three providers
- A rule-based event routing system using JSONPath expressions
- Automatic payload normalization
- Provider-aware SCM clients injected into your workers
- Multi-broker support (AMQP, NATS, Kafka, SQL, HTTP, etc.)

**Architecture Overview:**

githook consists of two main components:
1. **Server**: Receives webhooks, validates signatures, evaluates rules, and publishes events to message brokers
2. **Worker SDK**: Consumes events from brokers with provider-aware API clients pre-configured and ready to use

---

## How It Works

```
┌─────────────┐      ┌──────────────┐      ┌─────────────┐      ┌─────────────┐
│   GitHub    │      │              │      │   Message   │      │   Workers   │
│   GitLab    │─────▶│  githook    │─────▶│   Broker    │─────▶│  (Your App) │
│  Bitbucket  │      │   Server     │      │   (AMQP)    │      │             │
└─────────────┘      └──────────────┘      └─────────────┘      └─────────────┘
   Webhooks           Rules Engine          Watermill           Business Logic
                      + Publishing                              + SCM Clients
```

### Workflow

1. **Webhook Received**: A webhook arrives from GitHub, GitLab, or Bitbucket at the configured endpoint (e.g., `/webhooks/github`)

2. **Signature Validation**: The server validates the webhook signature using the provider's secret to ensure authenticity

3. **Payload Normalization**: The raw JSON payload is parsed and normalized into a common structure

4. **Rule Evaluation**: Each configured rule is evaluated against the normalized payload using JSONPath expressions and boolean logic

5. **Event Publishing**: If a rule matches, the event is published to the configured message broker(s) with the specified topic name

6. **Worker Consumption**: Workers subscribe to topics, receive events, and execute business logic with provider-aware API clients (GitHub SDK, GitLab SDK, Bitbucket SDK) automatically injected

7. **API Interactions**: Workers can interact with the provider's API using the injected client, which is pre-authenticated using GitHub App installation tokens or OAuth tokens

### Component Interaction

- **CLI**: Manage provider instances, list installations, configure the system
- **Server**: HTTP server that receives webhooks and publishes to brokers
- **Storage**: PostgreSQL database storing OAuth tokens and installation metadata
- **Message Brokers**: AMQP, NATS, Kafka, SQL, HTTP, or any Watermill-supported broker
- **Worker SDK**: Go library for consuming events with provider clients injected

---

## Features

- **Multi-Provider Support**: GitHub, GitLab, Bitbucket webhooks unified
- **Rule Engine**: JSONPath + boolean expressions for event routing
- **Multi-Broker Publishing**: AMQP, NATS, Kafka, HTTP, SQL, GoChannel, RiverQueue
- **API-First Architecture**: Connect RPC (gRPC) API for all operations
- **Multi-Tenant Ready**: Provider instance management with OAuth onboarding
- **Worker SDK**: Go SDK with auto-injected provider API clients
- **SCM Client Injection**: Pre-authenticated GitHub, GitLab, Bitbucket clients
- **Event Normalization**: Common payload structure across providers
- **Request Tracing**: End-to-end tracing with `X-Request-ID`

---

## Why githook

**Stop reinventing the wheel.** Every company builds the same webhook infrastructure over and over. We built it once, so you can reuse it.

- **Unified Webhooks**: One platform for GitHub, GitLab, and Bitbucket
- **Declarative Routing**: JSONPath rules instead of hardcoded logic
- **API-First Design**: Connect RPC (gRPC) API for programmatic control
- **Multi-Tenant**: Support multiple organizations with isolated configurations
- **Broker Agnostic**: AMQP, NATS, Kafka, SQL, HTTP - use any broker
- **Auto-Authenticated**: Workers get pre-configured API clients
- **Event-Driven**: Decouple webhook processing from business logic

---

## Installing the CLI

### Homebrew (macOS/Linux)

```bash
brew install yindia/homebrew-yindia/githook
```

### Install Script (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/yindia/githook/refs/heads/main/install.sh | sh
```

### From Source

```bash
git clone https://github.com/yindia/githook.git
cd githook
go build -o githook ./main.go
```

### Verify Installation

```bash
githook --version
```

---

## Quick Start Guide (GitHub Apps)

Get githook running locally with GitHub Apps in 4 steps:

### Prerequisites

- **Go 1.24+**
- **Docker + Docker Compose**
- **PostgreSQL** (started via Docker Compose)
- **RabbitMQ** (started via Docker Compose)
- **ngrok** (for local development - [download here](https://ngrok.com/download))
- **GitHub App** (create one at https://github.com/settings/apps)

### Step 1: Start Dependencies

```bash
docker compose up -d
```

This starts PostgreSQL and RabbitMQ.

### Step 2: Expose Local Server with ngrok

For local development, use ngrok to expose your local server to the internet:

```bash
ngrok http 8080
```

Copy the HTTPS forwarding URL (e.g., `https://abc123.ngrok-free.app`) - you'll need this for the next steps.

**Note:** Keep this ngrok terminal running throughout your testing session.

### Step 3: Configure GitHub App

Create a GitHub App with these settings:
- **Webhook URL**: `https://<your-ngrok-url>/webhooks/github` (use the URL from ngrok)
- **Webhook Secret**: `devsecret` (for local testing)
- **Permissions**: Repository metadata (read), Pull requests (read & write)
- **Subscribe to events**: Pull request, Push, Check suite

Download the private key and note your App ID.

### Step 4: Configure githook

Edit `config.yaml` and replace `<your-ngrok-url>` with your actual ngrok URL:

```yaml
server:
  port: 8080
endpoint: https://<your-ngrok-url>

storage:
  driver: postgres
  dsn: postgres://githook:githook@localhost:5432/githook?sslmode=disable
  dialect: postgres
  auto_migrate: true

auth:
  oauth2:
    enabled: false

redirect_base_url: https://app.example.com/success

```

> The per-provider configuration (app IDs, webhook secrets, driver settings) is no longer stored in `config.yaml`. Use the `githook` CLI to bootstrap providers/drivers/rules before the worker starts.

#### Bootstrap with the CLI

1. **Create a driver** (e.g., AMQP) so rules can publish to your broker:

```bash
githook --endpoint http://localhost:8080 drivers set \
  --tenant-id default \
  --name amqp \
  --config-file drivers/amqp.yaml
```

`drivers/amqp.yaml`:

```yaml
url: amqp://guest:guest@localhost:5672/
mode: durable_queue
```

2. **Create a provider instance** with OAuth/webhook metadata:

```bash
githook --endpoint http://localhost:8080 providers set \
  --tenant-id default \
  --provider github \
  --config-file providers/github.yaml
```

`providers/github.yaml` can include the `redirect_base_url` field:

```yaml
redirect_base_url: https://app.example.com/oauth/complete
app:
  app_id: 12345
  private_key_path: ./github.pem
webhook:
  secret: your-webhook-secret
oauth:
  client_id: "..."
  client_secret: "..."
```

After OAuth completes, users are redirected to this URL with query parameters containing the installation details. If not specified, the global `redirect_base_url` from server config is used.

3. **Create a rule** that emits a single topic and points at the driver:

```bash
githook --endpoint http://localhost:8080 rules create \
  --tenant-id default \
  --driver-id default:amqp \
  --when 'action == "opened"' \
  --emit github.main
```

4. **List resources** to confirm:

```bash
githook --endpoint http://localhost:8080 providers list
githook --endpoint http://localhost:8080 drivers list
githook --endpoint http://localhost:8080 rules list
```

Each rule must emit exactly one topic, which the worker will subscribe to using the rule’s `emit` value and resolved driver ID.


### Step 5: Start the Server

```bash
go run ./main.go serve --config config.yaml
```

The server should start on `http://localhost:8080` and be accessible via your ngrok URL.

### Step 6: Start a Worker

In another terminal (replace `RULE_ID` with a rule you created through the API):

```bash
go run ./example/github/worker/main.go --rule-id RULE_ID \
  --endpoint https://<your-ngrok-url>
```

### Step 7: Install the GitHub App

Get the provider instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider github
# Copy the instance hash (e.g., a1b2c3d4)
```

Visit the OAuth installation URL:
```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

Follow the GitHub authorization flow to complete installation. See [OAuth Onboarding Flow](#oauth-onboarding-flow) for detailed steps.

### Step 8: Trigger Events

Now test the integration by performing actions on an installed repository:

**Create a Pull Request:**
1. Open a repository where the GitHub App is installed
2. Create a new branch and make some changes
3. Open a pull request
4. The worker should log: `PR opened: github/pull_request`

**Push a Commit:**
1. Make a commit and push to the repository
2. The worker should log: `github.commit.created: repo=owner/repo commit=abc123...`

**Troubleshooting:**
- If webhooks aren't being received, check that ngrok is still running
- Verify your `endpoint` in `config.yaml` matches your ngrok URL
- Check GitHub App webhook delivery logs in GitHub App settings → Advanced → Recent Deliveries
- Ensure OAuth credentials (`client_id` and `client_secret`) are configured in `config.yaml`

---

## OAuth Onboarding Flow

OAuth onboarding allows users to connect their GitLab or Bitbucket accounts (or GitHub with user authorization) to githook.

### When to Use

- **GitLab**: Required for all GitLab integrations
- **Bitbucket**: Required for all Bitbucket integrations
- **GitHub**: Optional (only if "Request user authorization" is enabled in GitHub App)

### Configuration

Configure OAuth credentials and redirect URL:

```yaml
endpoint: https://your-domain.com  # Your public URL

redirect_base_url: https://app.example.com/success  # Global fallback for post-OAuth redirect
```

You can also set a per-provider-instance redirect URL in the config file:

```json
{
  "redirect_base_url": "https://app.example.com/oauth/github/complete",
  "app": { ... },
  "oauth": { ... }
}
```

Per-instance redirect URLs take precedence over the global `redirect_base_url` setting.

**Callback URLs** (configure in provider settings):
- **GitHub**: `https://your-domain.com/auth/github/callback`
- **GitLab**: `https://your-domain.com/auth/gitlab/callback`
- **Bitbucket**: `https://your-domain.com/auth/bitbucket/callback`

### How It Works

**Step 1: Get Instance Hash**

```bash
githook --endpoint https://your-domain.com providers list --provider github
# Output: Instance: a1b2c3d4
```

**Step 2: Redirect User to OAuth URL**

```
https://your-domain.com/?provider=github&instance=a1b2c3d4
```

**Step 3: Complete Authorization**

1. User is redirected to provider (GitHub/GitLab/Bitbucket)
2. User authorizes the application
3. Provider redirects back to githook with authorization code
4. githook exchanges code for access token
5. Token stored in PostgreSQL
6. User redirected to `redirect_base_url`

**Step 4: Done!**

- ✅ Installation created in database
- ✅ Webhooks will be processed
- ✅ Workers get authenticated API clients
- ✅ User redirected to `redirect_base_url` (if configured)

### Flow Diagram

```
User → githook → Provider OAuth → Callback → Store Token → Redirect to App
```

See [docs/oauth-callbacks.md](docs/oauth-callbacks.md) for detailed OAuth documentation.

---

## SCM-Specific Documentation

Detailed setup guides for each supported Git provider:

- **[GitHub Setup Guide](docs/getting-started-github.md)** - GitHub Apps, OAuth, webhook configuration
- **[GitLab Setup Guide](docs/getting-started-gitlab.md)** - OAuth application, webhook setup, namespaces
- **[Bitbucket Setup Guide](docs/getting-started-bitbucket.md)** - OAuth consumer, webhook configuration

Each guide includes:
- Provider-specific prerequisites
- Step-by-step configuration
- Webhook payload examples
- Testing instructions

---

## Terminology

### Providers
Git platforms: `github`, `gitlab`, or `bitbucket`.

### Provider Instances
Specific configurations of a provider (e.g., GitHub.com vs GitHub Enterprise). Each instance has a unique hash (e.g., `a1b2c3d4`) and separate credentials.

### Installation
The relationship between a provider instance, an account (org/user), and authentication credentials.

### Account ID
Unique identifier for the organization or user:
- **GitHub**: Login/username (e.g., `octocat`)
- **GitLab**: Group/user ID
- **Bitbucket**: Workspace slug

### Namespaces
Organizational units within a provider:
- **GitHub**: Organizations and user accounts
- **GitLab**: Groups and subgroups (hierarchical)
- **Bitbucket**: Workspaces

Webhooks can be configured at namespace level (affects all repos) or individual repository level.

### Drivers
Message brokers: `amqp`, `nats`, `kafka`, `sql`, `http`, `gochannel`, `riverqueue`.

### Rules
JSONPath conditions that route events to topics. Has `when` (condition), `emit` (topic), and optional `drivers`.

### Topics
Message broker queues/subjects that events are published to. Workers subscribe to topics.

---

## Storage

githook uses PostgreSQL to persist OAuth tokens, GitHub App installation metadata, and provider instance configurations.

```yaml
storage:
  driver: postgres
  dsn: postgres://githook:githook@localhost:5432/githook?sslmode=disable
  dialect: postgres
  auto_migrate: true
```

See [docs/storage.md](docs/storage.md) for advanced storage configuration.

---

## Webhook URLs

Webhook URL schema: `<base-url>/webhooks/<provider>`

**Default webhook paths:**
- **GitHub:** `/webhooks/github`
- **GitLab:** `/webhooks/gitlab`
- **Bitbucket:** `/webhooks/bitbucket`

---

## Drivers

Drivers are message broker implementations that githook uses to publish events. Powered by [Watermill](https://watermill.io/), githook supports multiple brokers simultaneously.

### Available Drivers

**AMQP (RabbitMQ)**
```yaml
watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue
```

Modes:
- `durable_queue`: Persistent queues (survives broker restart)
- `nondurable_queue`: Ephemeral queues
- `durable_pubsub`: Persistent topic exchanges
- `nondurable_pubsub`: Ephemeral topic exchanges

**NATS Streaming**
```yaml
watermill:
  driver: nats
  nats:
    url: nats://localhost:4222
    cluster_id: test-cluster
    client_id: githook-publisher
```

**Kafka**
```yaml
watermill:
  driver: kafka
  kafka:
    brokers: ["localhost:9092"]
    consumer_group: githook
```

**SQL (PostgreSQL/MySQL)**
```yaml
watermill:
  driver: sql
  sql:
    driver: postgres
    dsn: postgres://user:pass@localhost/db?sslmode=disable
    table: events
```

**HTTP**
```yaml
watermill:
  driver: http
  http:
    mode: base_url
    base_url: http://localhost:9000/hooks
```

**GoChannel (In-Memory)**
```yaml
watermill:
  driver: gochannel
```

Use for testing or single-binary deployments.

**RiverQueue (Postgres Job Queue)**
```yaml
watermill:
  driver: riverqueue
  riverqueue:
    driver: postgres
    dsn: postgres://user:pass@localhost:5432/db?sslmode=disable
    table: river_job
    queue: default
    kind: githook.event
```

### Multi-Driver Fan-Out

Publish the same event to multiple brokers:

```yaml
watermill:
  drivers: [amqp, http, sql]
  amqp:
    url: amqp://guest:guest@localhost:5672/
  http:
    base_url: http://localhost:9000/hooks
  sql:
    driver: postgres
    dsn: postgres://user:pass@localhost/db
```

### Per-Rule Driver Targeting

Override drivers for specific rules:

```yaml
rules:
  - when: action == "opened"
    emit: pr.opened
    drivers: [amqp]  # Only publish to AMQP

  - when: action == "closed"
    emit: pr.closed
    drivers: [amqp, http]  # Publish to both AMQP and HTTP
```

If `drivers` is omitted, the default `driver` or `drivers` from the Watermill config is used.

See [docs/drivers.md](docs/drivers.md) for advanced driver configuration.

---

## Rules

Rules are the heart of githook' event routing system. They define which webhook events to publish and where to send them.

### Rule Structure

```yaml
rules:
  - when: <boolean-expression>
    emit: <topic-name>
    drivers: [<driver-list>]  # optional
```

### Rule Evaluation

1. A webhook is received and validated
2. The payload is normalized
3. Each rule's `when` condition is evaluated against the normalized payload
4. If the condition is `true`, the event is published to the `emit` topic
5. Multiple rules can match the same event (multi-match)

### JSONPath Expressions

The `when` field uses JSONPath with boolean operators:

**Simple field access:**
```yaml
when: action == "opened"
```

**Nested fields:**
```yaml
when: pull_request.draft == false
```

**Full JSONPath syntax:**
```yaml
when: $.pull_request.head.ref == "main"
```

**Boolean operators:**
```yaml
when: action == "opened" && pull_request.draft == false
when: action == "closed" || action == "merged"
when: pull_request.draft != true
```

**Helper functions:**
```yaml
# Check if string contains substring
when: contains(pull_request.title, "[WIP]")

# Pattern matching with wildcards
when: like(pull_request.head.ref, "feature/%")
```

### Multi-Topic Publishing

Publish the same event to multiple topics:

```yaml
rules:
  - when: action == "closed" && pull_request.merged == true
    emit: [pr.merged, audit.pr.merged, notifications.pr.merged]
```

### Strict Mode

By default, rules with missing fields won't match. Enable strict mode to fail loudly:

```yaml
rules_strict: true
rules:
  - when: nonexistent.field == "value"
    emit: will.never.match
```

### Example Rules

**Pull request events:**
```yaml
rules:
  # Non-draft PR opened
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready

  # PR merged
  - when: action == "closed" && pull_request.merged == true
    emit: pr.merged

  # PR draft converted to ready
  - when: action == "ready_for_review"
    emit: pr.ready_for_review
```

**Commit/push events:**
```yaml
rules:
  # Single commit pushed
  - when: head_commit.id != "" && commits[0].id != "" && commits[1] == null
    emit: github.commit.created

  # Check suite requested (also contains commit info)
  - when: action == "requested" && check_suite.head_commit.id != ""
    emit: github.commit.created
```

**Tag events:**
```yaml
rules:
  - when: ref_type == "tag"
    emit: github.tag.created
```

**Branch-specific rules:**
```yaml
rules:
  - when: pull_request.base.ref == "main"
    emit: pr.targeting.main

  - when: like(pull_request.head.ref, "release/%")
    emit: pr.from.release.branch
```

See [docs/rules.md](docs/rules.md) for advanced rule patterns.

---

## SDK

The githook Worker SDK provides a Go library for consuming events from message brokers with provider-aware API clients injected.

### Installation

```bash
go get githook/sdk/go/worker
```

### Basic Usage

```go
package main

import (
    "context"
    "log"
    "os"

    "githook/sdk/go/worker"
)

func main() {
    wk := worker.New(
        worker.WithEndpoint(os.Getenv("GITHOOK_ENDPOINT")),
        worker.WithAPIKey(os.Getenv("GITHOOK_API_KEY")),
        worker.WithDefaultDriver("driver-id"),
        worker.WithTopics("pr.opened.ready", "pr.merged"),
    )

    wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
        log.Printf("PR opened: %s/%s", evt.Provider, evt.Type)
        return nil
    })

    if err := wk.Run(context.Background()); err != nil {
        log.Fatal(err)
    }
}
```

### Event Structure

```go
type Event struct {
    Topic      string                 // Topic name (e.g., "pr.opened.ready")
    Provider   string                 // Provider name ("github", "gitlab", "bitbucket")
    Type       string                 // Event type (e.g., "pull_request")
    Payload    []byte                 // Raw JSON payload from webhook
    Normalized map[string]interface{} // Normalized payload
    Metadata   map[string]string      // Additional metadata (driver, request ID)
    Client     interface{}            // Provider API client (if configured)
}
```

### Using Provider Clients

The SDK automatically injects authenticated API clients:

**GitHub:**
```go
wk.HandleTopic("pr.merged", "driver-id", func(ctx context.Context, evt *worker.Event) error {
    if evt.Provider != "github" {
        return nil
    }

    if evt.Client != nil {
        ghClient := evt.Client.(*github.Client)

        // Use the GitHub SDK
        repo, _, err := ghClient.Repositories.Get(ctx, "owner", "repo")
        if err != nil {
            return err
        }

        log.Printf("Repository: %s, Stars: %d", repo.GetName(), repo.GetStargazersCount())
    }

    return nil
})
```

**GitLab:**
```go
wk.HandleTopic("gitlab.mr.opened", "driver-id", func(ctx context.Context, evt *worker.Event) error {
    if evt.Client != nil {
        glClient := evt.Client.(*gitlab.Client)
        // Use GitLab SDK
    }
    return nil
})
```

**Bitbucket:**
```go
wk.HandleTopic("bitbucket.pr.opened", "driver-id", func(ctx context.Context, evt *worker.Event) error {
    if evt.Client != nil {
        bbClient := evt.Client.(*bitbucket.Client)
        // Use Bitbucket SDK
    }
    return nil
})
```

### Client Provider Configuration

Enable client injection by passing provider config:

```go
appCfg, _ := core.LoadConfig("config.yaml")

wk := worker.New(
    worker.WithSubscriber(sub),
    worker.WithTopics("pr.opened.ready"),
    worker.WithClientProvider(worker.NewSCMClientProvider(appCfg.Providers)),
)
```

### Concurrency

Control how many events are processed simultaneously:

```go
wk := worker.New(
    worker.WithConcurrency(20), // Process 20 events in parallel
)
```

### Middleware

Use Watermill middleware for retry, logging, throttling:

```go
import wmmw "github.com/ThreeDotsLabs/watermill/message/router/middleware"

retryMiddleware := worker.MiddlewareFromWatermill(
    wmmw.Retry{MaxRetries: 3}.Middleware,
)

wk := worker.New(
    worker.WithMiddleware(retryMiddleware),
)
```

### Error Handling

Implement custom retry logic:

```go
type retryOnce struct{}

func (retryOnce) OnError(ctx context.Context, evt *worker.Event, err error) worker.RetryDecision {
    // Retry once, then nack
    return worker.RetryDecision{Retry: true, Nack: true}
}

wk := worker.New(
    worker.WithRetry(retryOnce{}),
)
```

### Lifecycle Hooks

```go
wk := worker.New(
    worker.WithListener(worker.Listener{
        OnStart: func(ctx context.Context) {
            log.Println("Worker started")
        },
        OnExit: func(ctx context.Context) {
            log.Println("Worker stopped")
        },
        OnError: func(ctx context.Context, evt *worker.Event, err error) {
            log.Printf("Error processing event: %v", err)
        },
        OnMessageFinish: func(ctx context.Context, evt *worker.Event, err error) {
            log.Printf("Finished: provider=%s type=%s err=%v", evt.Provider, evt.Type, err)
        },
    }),
)
```

### Worker invocation

Workers already resolve the driver and topic from a rule, so you only need the rule ID:

```bash
go run ./worker/main.go --rule-id RULE_ID --endpoint https://your-domain.com
```

Use `subCfg.RuleID` or custom handler wiring when building reusable worker logic, and consult [docs/sdk_clients.md](docs/sdk_clients.md) for advanced SDK helpers (HTTP clients, middleware, API clients, etc.).
## Documentation

## Architecture: Control Plane vs Data Plane

Githooks is split into a **control plane** and a **data plane**:

- **Control Plane (Server)**: Hosts the Connect RPC API, stores configuration and installation data, manages rules, and publishes events to the message bus. This is the source of truth for providers, drivers, and rules.
- **Data Plane (Worker)**: Subscribes to topics and processes events. It resolves provider clients via the server API and focuses on business logic. Workers should not access platform storage directly.

This separation lets you scale event processing independently while keeping configuration centralized.

### Configuration Guides
- [API Reference](https://buf.build/githook/cloud) - Connect RPC API documentation
- [Driver Configuration](docs/drivers.md) - Message broker setup
- [Rules Engine](docs/rules.md) - Event routing patterns
- [Storage](docs/storage.md) - Database configuration
- [OAuth Callbacks](docs/oauth-callbacks.md) - OAuth flow details
- [Webhook Setup](docs/webhooks.md) - Provider webhook configuration

### Provider Guides
- [GitHub Setup](docs/getting-started-github.md) - GitHub Apps, OAuth, webhooks
- [GitLab Setup](docs/getting-started-gitlab.md) - GitLab OAuth, webhooks, namespaces
- [Bitbucket Setup](docs/getting-started-bitbucket.md) - Bitbucket OAuth, webhooks

### SDK & Integration
- [SDK Client Injection](docs/sdk_clients.md) - Using provider API clients in workers
- [SDK DSL](docs/sdk-dsl.md) - Portable worker specification
- [CLI Usage](docs/cli.md) - Command-line interface reference

### Advanced Topics
- [API Authentication](docs/auth.md) - OAuth2/OIDC for Connect RPC
- [SCM Authentication](docs/scm-auth.md) - GitHub App, GitLab/Bitbucket tokens
- [Event Compatibility](docs/events.md) - Event payload formats
- [Observability](docs/observability.md) - Logging, metrics, tracing

---

**Made with ❤️ for developers who automate Git workflows**

Questions? Issues? Check the [documentation](docs/) or [open an issue](https://github.com/yindia/githook/issues).

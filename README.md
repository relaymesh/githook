# Githooks ⚡

> **⚠️ Warning:** This project is for research and development only and is **not production-ready**. Do not deploy it in production environments.

## Table of Contents

1. [About](#about)
2. [How It Works](#how-it-works)
3. [Features](#features)
4. [Why Githooks](#why-githooks)
5. [Installing the CLI](#installing-the-cli)
6. [Quick Start Guide (GitHub Apps)](#quick-start-guide-github-apps)
7. [SCM-Specific Documentation](#scm-specific-documentation)
8. [Authentication](#authentication)
9. [Terminology](#terminology)
10. [Storage](#storage)
11. [OAuth Callbacks and Webhook URLs](#oauth-callbacks-and-webhook-urls)
12. [Drivers](#drivers)
13. [Rules](#rules)
14. [SDK](#sdk)
15. [Examples](#examples)
16. [Documentation](#documentation)

---

## About

Githooks is an event automation layer for GitHub, GitLab, and Bitbucket. It receives webhooks from these providers, evaluates configurable rules against the payload, and publishes matching events to your message broker using [Watermill](https://watermill.io/).

**What problem does it solve?**

Managing webhooks across multiple Git providers (GitHub, GitLab, Bitbucket) typically requires:
- Writing provider-specific webhook handlers for each platform
- Manually normalizing different payload formats
- Hardcoding event routing logic in your application
- Managing authentication for each provider's API separately

Githooks solves this by providing:
- A unified webhook receiver for all three providers
- A rule-based event routing system using JSONPath expressions
- Automatic payload normalization
- Provider-aware SCM clients injected into your workers
- Multi-broker support (AMQP, NATS, Kafka, SQL, HTTP, etc.)

**Architecture Overview:**

Githooks consists of two main components:
1. **Server**: Receives webhooks, validates signatures, evaluates rules, and publishes events to message brokers
2. **Worker SDK**: Consumes events from brokers with provider-aware API clients pre-configured and ready to use

---

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

- **Multi-Provider Webhooks**: Unified webhook handling for GitHub, GitLab, and Bitbucket
- **Rule Engine**: JSONPath-based boolean expressions with multi-match support
- **Flexible Publishing**: Publish to AMQP, NATS, Kafka, HTTP, SQL, GoChannel, RiverQueue
- **Multi-Driver Fan-Out**: Publish the same event to multiple brokers simultaneously
- **Event Normalization**: Common payload structure across all providers
- **Protobuf Event Envelope**: Events use `cloud.v1.EventPayload` with raw JSON preserved
- **Worker SDK**: Built-in concurrency, middleware, graceful shutdown
- **SCM Client Injection**: GitHub, GitLab, Bitbucket API clients automatically configured
- **Multi-Tenant Support**: Provider instance management with OAuth onboarding
- **GitHub App Authentication**: JWT generation → installation token exchange
- **OAuth Token Storage**: Secure token persistence in PostgreSQL
- **Request Tracing**: `X-Request-ID` header for end-to-end request tracking
- **Observability**: Structured logging with provider, event type, and installation context

---

## Why Githooks

**Unified Event Handling**
- Write once, handle webhooks from GitHub, GitLab, and Bitbucket without provider-specific code
- No need to maintain separate webhook endpoints and handlers for each platform

**Declarative Event Routing**
- Define rules using JSONPath and boolean expressions instead of hardcoding routing logic
- Change event routing by editing YAML config instead of deploying new code

**Broker Flexibility**
- Use any message broker supported by Watermill (AMQP, NATS, Kafka, SQL, HTTP)
- Fan-out events to multiple brokers for redundancy or different consumers

**Provider-Aware Clients**
- Workers receive pre-authenticated API clients for the event's provider
- No manual token management, JWT signing, or client initialization

**Multi-Tenant Architecture**
- Support multiple organizations or teams with instance-based provider configuration
- OAuth onboarding flow for easy installation setup

**Event-Driven Workflows**
- Decouple webhook handling from business logic execution
- Scale workers independently from the webhook receiver
- Retry failed events using broker capabilities

---

## Installing the CLI

### Homebrew (macOS/Linux)

```bash
brew install yindia/homebrew-yindia/githooks
```

### Install Script (Linux/macOS)

```bash
curl -fsSL https://raw.githubusercontent.com/yindia/githooks/refs/heads/main/install.sh | sh
```

### From Source

```bash
git clone https://github.com/yindia/githooks.git
cd githooks
go build -o githooks ./main.go
```

### Verify Installation

```bash
githooks --version
```

---

## Quick Start Guide (GitHub Apps)

Get Githooks running locally with GitHub Apps in 4 steps:

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

### Step 4: Configure Githooks

Edit `config.yaml` and replace `<your-ngrok-url>` with your actual ngrok URL:

```yaml
server:
  port: 8080
  public_base_url: https://<your-ngrok-url>

providers:
  github:
    enabled: true
    webhook:
      secret: devsecret
    app:
      app_id: 123456
      private_key_path: /path/to/github.pem
      app_slug: your-app-slug
    api:
      base_url: https://api.github.com
    oauth:
      client_id: ${GITHUB_OAUTH_CLIENT_ID}
      client_secret: ${GITHUB_OAUTH_CLIENT_SECRET}
      scopes: ["read:user"]

watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue

storage:
  driver: postgres
  dsn: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
  dialect: postgres
  auto_migrate: true

rules:
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready
  - when: action == "closed" && pull_request.merged == true
    emit: pr.merged
  # Push events (single commit)
  - when: head_commit.id != "" && commits[0].id != "" && commits[1] == null
    emit: github.commit.created
  # Check suite events (also contain commit information)
  - when: action == "requested" && check_suite.head_commit.id != ""
    emit: github.commit.created
```

### Step 5: Start the Server

```bash
export GITHUB_WEBHOOK_SECRET=devsecret
go run ./main.go serve --config config.yaml
```

The server should start on `http://localhost:8080` and be accessible via your ngrok URL.

### Step 6: Start a Worker

In another terminal:

```bash
go run ./example/github/worker/main.go --config config.yaml --driver amqp
```

### Step 7: Test with Webhooks

Test different event types that match the configured rules:

**Pull request opened:**
```bash
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

**Commit/push event:**
```bash
./scripts/send_webhook.sh github push example/github/push.json
```

You should see the worker receive and process these events in the terminal.

### Step 8: Install the GitHub App

Install your GitHub App on a repository and open a pull request. The worker will receive the event automatically.

**Troubleshooting:**
- If webhooks aren't being received, check that ngrok is still running
- Verify your `public_base_url` in `config.yaml` matches your ngrok URL
- Check GitHub App webhook delivery logs in GitHub settings

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

## Authentication

Githooks supports different authentication methods for each provider.

### GitHub Authentication

**GitHub App (Recommended)**
- Create a GitHub App with webhook and API permissions
- Githooks generates JWT tokens signed with your private key
- Exchanges JWT for installation tokens scoped to specific repositories
- Installation tokens are cached and automatically refreshed

**OAuth (Optional)**
- Used for user-initiated onboarding flows
- Requires OAuth client ID and secret
- Tokens stored in PostgreSQL after authorization

### GitLab Authentication

**OAuth Application**
- Create a GitLab OAuth application
- Configure callback URL: `https://your-domain.com/auth/gitlab/callback`
- Tokens obtained during OAuth flow are stored in PostgreSQL
- Tokens used for API calls to fetch repositories and namespaces

### Bitbucket Authentication

**OAuth Consumer**
- Create a Bitbucket OAuth consumer
- Configure callback URL: `https://your-domain.com/auth/bitbucket/callback`
- OAuth 2.0 tokens stored after user authorization
- Tokens used for Bitbucket API interactions

### API Authentication (Optional)

Secure Connect RPC endpoints with OAuth2/OIDC:

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://your-okta-domain/oauth2/default
    audience: api://githooks
```

When enabled, all CLI and API calls require a bearer token. Webhooks and `/auth/*` endpoints remain public.

See [docs/auth.md](docs/auth.md) for detailed authentication setup.

---

## Terminology

Understanding these key concepts will help you configure and use Githooks effectively.

### Providers

A **provider** is a Git platform integration: `github`, `gitlab`, or `bitbucket`. Each provider has its own webhook format, authentication method, and API client.

### Provider Instances

A **provider instance** represents a specific configuration of a provider. For example:
- `github.com` (public GitHub)
- GitHub Enterprise Server at `ghe.company.com`
- GitLab SaaS vs self-hosted GitLab

Each instance has:
- A unique **instance hash** (server-generated, e.g., `a1b2c3d4`)
- Separate API base URLs
- Separate OAuth credentials
- Separate webhook secrets

**Why instances?** Multi-tenant setups or organizations using both public and self-hosted platforms need to configure multiple instances of the same provider.

### Installation

An **installation** represents the relationship between:
- A provider instance (e.g., GitHub)
- An account (organization or user, e.g., `octocat`)
- Authentication credentials (OAuth token or GitHub App installation ID)

Installations are created when:
1. A GitHub App is installed on an organization/user
2. A user completes the OAuth flow for GitLab or Bitbucket

### Account ID

The **account ID** is the unique identifier for the organization or user on the provider platform:
- GitHub: organization login or username (e.g., `octocat`)
- GitLab: group ID or user ID
- Bitbucket: workspace slug

### Installation ID

The **installation ID** is provider-specific:
- **GitHub**: Numeric installation ID assigned when a GitHub App is installed (e.g., `12345678`)
- **GitLab**: Not applicable (uses account ID)
- **Bitbucket**: Not applicable (uses account ID)

### State ID

A **state ID** is a cryptographically random string used for CSRF protection during OAuth flows. It's generated when initiating OAuth and validated when the callback is received.

### Namespaces

**Namespaces** are organizational units within a provider:
- **GitHub**: Organizations and users
- **GitLab**: Groups and subgroups
- **Bitbucket**: Workspaces

During OAuth onboarding, Githooks fetches available namespaces and allows users to select which ones to grant access to.

### Drivers

**Drivers** are message broker implementations supported by Watermill:
- `amqp`: RabbitMQ, ActiveMQ
- `nats`: NATS Streaming
- `kafka`: Apache Kafka
- `sql`: PostgreSQL, MySQL (as a queue)
- `http`: HTTP POST to an endpoint
- `gochannel`: In-memory Go channels (for testing or single-binary deployments)
- `riverqueue`: River job queue (Postgres-based)

### Rules

**Rules** are JSONPath-based conditions that determine which events to publish and where. Each rule has:
- `when`: Boolean expression evaluated against webhook payload
- `emit`: Topic name(s) to publish to if condition matches
- `drivers`: (Optional) Override which brokers to publish to

### Topics

**Topics** are message broker subjects/queues that events are published to. Workers subscribe to topics to receive events. Topic names are defined in rules (the `emit` field).

---

## Storage

Githooks uses PostgreSQL to persist OAuth tokens, GitHub App installation metadata, and provider instance configurations.

### Database Schema

The storage layer manages these entities:

**Installations Table:**
- Stores provider installations with OAuth tokens
- Indexed by: `tenant_id`, `provider`, `account_id`, `installation_id`, `instance_key`
- Columns: tokens, account name, created/updated timestamps

**Provider Instances:**
- Stored on server startup from config file
- Each instance gets a server-generated hash (e.g., `a1b2c3d4`)
- Used to differentiate between multiple configs of the same provider

### Configuration

```yaml
storage:
  driver: postgres
  dsn: postgres://githooks:githooks@localhost:5432/githooks?sslmode=disable
  dialect: postgres
  auto_migrate: true
```

**Options:**
- `driver`: Database driver (`postgres`, `mysql`, `sqlite`)
- `dsn`: Data source name (connection string)
- `dialect`: SQL dialect for GORM
- `auto_migrate`: Automatically create/update schema on startup

### Token Security

- OAuth access tokens and refresh tokens are stored in plaintext
- **Production recommendation**: Encrypt sensitive columns using database-level encryption or application-level encryption
- Consider using HashiCorp Vault or AWS Secrets Manager for token storage in production
- Rotate tokens periodically using OAuth refresh token flow

### State Management

- OAuth state parameters are ephemeral and not persisted
- Installation tokens (GitHub) are cached in memory and refreshed as needed
- Provider instance hashes are deterministic and regenerated on server restart

See [docs/storage.md](docs/storage.md) for advanced storage configuration.

---

## OAuth Callbacks and Webhook URLs

### OAuth Callback URLs

When configuring OAuth applications on each provider, use these callback paths.

**For Local Development (with ngrok):**

First, start ngrok to get your public URL:
```bash
ngrok http 8080
# Copy the HTTPS forwarding URL (e.g., https://abc123.ngrok-free.app)
```

Then configure these callback URLs in your provider settings:

- **GitHub:** `https://<your-ngrok-url>/auth/github/callback`
- **GitLab:** `https://<your-ngrok-url>/auth/gitlab/callback`
- **Bitbucket:** `https://<your-ngrok-url>/auth/bitbucket/callback`

**For Production:**

- **GitHub:** `https://your-domain.com/auth/github/callback`
- **GitLab:** `https://your-domain.com/auth/gitlab/callback`
- **Bitbucket:** `https://your-domain.com/auth/bitbucket/callback`

**Important:**
- The path must be `/auth/{provider}/callback` (not `/oauth/{provider}/callback`)
- Set `server.public_base_url` in your config to match your domain
- For local development with ngrok, run `ngrok http 8080` and use the generated URL

**Local Development Setup:**
```bash
# Start ngrok to expose your local server
ngrok http 8080

# Use the ngrok URL in your config
```

**Example config:**
```yaml
server:
  public_base_url: https://<your-ngrok-url>  # Replace with actual ngrok URL

oauth:
  redirect_base_url: https://app.example.com/oauth/complete
```

### Webhook URLs

Configure these webhook URLs in your provider settings.

**For Local Development (with ngrok):**

First, start ngrok:
```bash
ngrok http 8080
```

Then use the ngrok URL in your webhook configuration:

- **GitHub:** `https://<your-ngrok-url>/webhooks/github`
- **GitLab:** `https://<your-ngrok-url>/webhooks/gitlab`
- **Bitbucket:** `https://<your-ngrok-url>/webhooks/bitbucket`

**For Production:**

- **GitHub:** `https://your-domain.com/webhooks/github`
- **GitLab:** `https://your-domain.com/webhooks/gitlab`
- **Bitbucket:** `https://your-domain.com/webhooks/bitbucket`

**Custom webhook paths (optional):**
```yaml
providers:
  github:
    webhook:
      path: /custom/github/webhook
      secret: ${GITHUB_WEBHOOK_SECRET}
```

### OAuth Onboarding Flow

**Initiating OAuth:**

For local development, redirect users to your ngrok URL:
```
https://<your-ngrok-url>/?provider=github&instance=<instance-hash>
```

For production:
```
https://your-domain.com/?provider=github&instance=<instance-hash>
```

**Getting the instance hash:**

```bash
# Local development
githooks --endpoint http://localhost:8080 providers list --provider github

# Production
githooks --endpoint https://your-domain.com providers list --provider github
```

This returns all configured provider instances with their hashes.

**Multiple Provider Instances:**

If you have multiple instances (e.g., GitHub.com + GHE), specify which instance:
```
https://<your-domain>/?provider=github&instance=a1b2c3d4
```

**Notes:**
- GitHub App installs are initiated from the GitHub App installation page
- The GitHub OAuth callback is only used when "Request user authorization (OAuth)" is enabled in the GitHub App settings
- The callback receives authorization codes and exchanges them for access tokens
- Tokens are stored in PostgreSQL and used for API authentication
- For local development, ensure ngrok is running before initiating OAuth flows

See [docs/oauth-callbacks.md](docs/oauth-callbacks.md) for detailed callback flow documentation.

---

## Drivers

Drivers are message broker implementations that Githooks uses to publish events. Powered by [Watermill](https://watermill.io/), Githooks supports multiple brokers simultaneously.

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
    client_id: githooks-publisher
```

**Kafka**
```yaml
watermill:
  driver: kafka
  kafka:
    brokers: ["localhost:9092"]
    consumer_group: githooks
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
    kind: githooks.event
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

Rules are the heart of Githooks' event routing system. They define which webhook events to publish and where to send them.

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

The Githooks Worker SDK provides a Go library for consuming events from message brokers with provider-aware API clients injected.

### Installation

```bash
go get githooks/sdk/go/worker
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "githooks/sdk/go/worker"
)

func main() {
    // Load subscriber config from the same file the server uses
    subCfg, err := worker.LoadSubscriberConfig("config.yaml")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    sub, err := worker.BuildSubscriber(subCfg)
    if err != nil {
        log.Fatalf("Failed to build subscriber: %v", err)
    }
    defer sub.Close()

    wk := worker.New(
        worker.WithSubscriber(sub),
        worker.WithTopics("pr.opened.ready", "pr.merged"),
        worker.WithConcurrency(10),
    )

    wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
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
wk.HandleTopic("pr.merged", func(ctx context.Context, evt *worker.Event) error {
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
wk.HandleTopic("gitlab.mr.opened", func(ctx context.Context, evt *worker.Event) error {
    if evt.Client != nil {
        glClient := evt.Client.(*gitlab.Client)
        // Use GitLab SDK
    }
    return nil
})
```

**Bitbucket:**
```go
wk.HandleTopic("bitbucket.pr.opened", func(ctx context.Context, evt *worker.Event) error {
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

### Driver Override

Target a specific subscriber driver:

```bash
go run ./worker/main.go --config config.yaml --driver amqp
```

```go
subCfg.Driver = "amqp"
```

See [docs/sdk_clients.md](docs/sdk_clients.md) for advanced SDK usage.

---

## Examples

The `example/` directory contains working examples for different use cases:

### GitHub Example

**Path:** `example/github/`

Demonstrates:
- Handling GitHub webhooks
- Pull request events
- Commit/push events
- Using GitHub API client in workers

**Run:**
```bash
# Start worker
go run ./example/github/worker/main.go --config config.yaml --driver amqp

# Send test webhook
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

### GitLab Example

**Path:** `example/gitlab/`

Demonstrates:
- GitLab merge request events
- Push events
- Using GitLab API client

### Bitbucket Example

**Path:** `example/bitbucket/`

Demonstrates:
- Bitbucket pull request events
- Using Bitbucket API client

### RiverQueue Example

**Path:** `example/riverqueue/`

Demonstrates:
- Publishing events to River job queue (Postgres-based)
- Processing jobs with River workers

### Real-World Example

**Path:** `example/realworld/`

Demonstrates:
- Multiple workers consuming from the same topic
- Production-style error handling
- Retry logic and circuit breakers

### Vercel Example

**Path:** `example/vercel/`

Demonstrates:
- Preview deployment hooks
- Production deployment hooks
- Integration with Vercel API

### In-Process Example

**Path:** `example/inprocess/`

Demonstrates:
- Single-binary deployment with GoChannel driver
- Server and workers in the same process
- Useful for edge deployments or testing

---

## Documentation

### Configuration Guides
- [API Reference](https://buf.build/githooks/cloud) - Connect RPC API documentation
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

Questions? Issues? Check the [documentation](docs/) or [open an issue](https://github.com/yindia/githooks/issues).

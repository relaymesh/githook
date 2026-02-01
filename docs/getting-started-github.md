# Getting Started: GitHub

Let us build a working GitHub webhook pipeline end to end: start the broker stack, run the server, run a worker, and then plug in a real GitHub App so events flow from GitHub to your code.

## Before you begin

You will need:

- Go 1.21+
- Docker + Docker Compose
- A GitHub account

## Step 1: start the local broker stack

From the repo root:

```bash
docker compose up -d
```

This boots RabbitMQ, NATS Streaming, Kafka/Zookeeper, and Postgres. The server will publish to them based on your configuration.

## Step 2: run the webhook server

Use the GitHub example config:

```bash
go run ./main.go serve --config example/github/app.yaml
```

You should see logs showing the server is listening on `:8080`.

## Step 3: run the worker

In another terminal:

```bash
go run ./example/github/worker/main.go
```

The worker subscribes to topics emitted by the GitHub rules and logs each match.

## Step 4: send a local test event

Try a simulated pull request event:

```bash
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

Expected result:

- The server logs a rule match and publishes a topic.
- The worker logs that it handled the topic.

At this point, the full local loop works. Now let us wire real GitHub traffic into it.

## Step 5: create a GitHub App

1. GitHub: **Settings** -> **Developer settings** -> **GitHub Apps** -> **New GitHub App**.
2. App name: `githooks-local` (any name is fine).
3. Homepage URL: `http://localhost:8080`.
4. Webhook URL: `http://localhost:8080/webhooks/github`.
5. Webhook secret: pick a random string and save it.
6. Permissions:
   - Repository permissions: set **Pull requests** to **Read-only**.
7. Subscribe to events:
   - `Pull request`
   - `Push` (optional)
8. Create the app.

### Install the app on a repo

1. In the app settings, click **Install App**.
2. Choose a test repository and install.

Optional: you can also start the install flow by visiting `http://localhost:8080/?provider=github`, which redirects to the GitHub App install page when `providers.github.app.app_slug` is set.

### Update your config

Set the webhook secret in `example/github/app.yaml` or export it as an env var:

```yaml
providers:
  github:
    enabled: true
    webhook:
      secret: ${GITHUB_WEBHOOK_SECRET}
```

Then:

```bash
export GITHUB_WEBHOOK_SECRET="your-secret"
go run ./main.go serve --config example/github/app.yaml
```

GitHub will now deliver real webhook events to your local server.

## Step 6: expose localhost with ngrok (optional)

If GitHub cannot reach your machine:

```bash
ngrok http 8080
```

Update the GitHub App webhook URL to the ngrok URL.

## Troubleshooting

- `missing X-Hub-Signature`: your webhook secret does not match.
- `no matching rules`: ensure rules in `example/github/app.yaml` match your payload.
- `connection refused`: make sure Docker Compose is running for broker drivers.

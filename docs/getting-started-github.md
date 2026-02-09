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
6. **Callback URL** (if using OAuth): `http://localhost:8080/auth/github/callback`
   - This is required if you enable "Request user authorization (OAuth) during installation"
   - **Important**: The path must be `/auth/github/callback` (not `/oauth/github/callback`)
7. Permissions:
   - Repository permissions: set **Pull requests** to **Read-only**.
8. Subscribe to events:
   - `Pull request`
   - `Push` (optional)
9. Create the app.

### Install the app on a repo

1. In the app settings, click **Install App**.
2. Choose a test repository and install.

Optional: you can also start the install flow by visiting `http://localhost:8080/?provider=github`, which redirects to the GitHub App install page when `providers.github.app.app_slug` is set.

### Using Multiple Provider Instances

If you have multiple GitHub provider instances configured (e.g., GitHub.com and GitHub Enterprise), you need to specify which instance to use with the `instance` parameter:

```
http://localhost:8080/?provider=github&instance=<instance-key>
```

To get the instance key, run:

```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider github
```

This will show all configured GitHub provider instances and their corresponding instance keys (hashes).

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

Update the GitHub App configuration with your ngrok URL:
- **Webhook URL**: `https://your-ngrok-url.ngrok-free.app/webhooks/github`
- **Callback URL**: `https://your-ngrok-url.ngrok-free.app/auth/github/callback`

Also update your config file with the public base URL:

```yaml
server:
  public_base_url: https://your-ngrok-url.ngrok-free.app
```

## Troubleshooting

- `missing X-Hub-Signature`: your webhook secret does not match.
- `no matching rules`: ensure rules in `example/github/app.yaml` match your payload.
- `connection refused`: make sure Docker Compose is running for broker drivers.
- `404 page not found` on callback: verify the GitHub App callback URL is `/auth/github/callback` (not `/oauth/github/callback`).
- `database constraint error`: drop and recreate the `githooks_installations` table if you upgraded from an older version.

# Getting Started: GitHub

Build a working GitHub webhook pipeline: start the broker stack, run the server, run a worker, and connect a real GitHub App so events flow from GitHub to your code.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- ngrok (for local development - [download here](https://ngrok.com/download))
- A GitHub account

## Step 1: Start Dependencies

```bash
docker compose up -d
```

This starts PostgreSQL and RabbitMQ.

## Step 2: Expose with ngrok

```bash
ngrok http 8080
```

Copy the HTTPS forwarding URL (e.g., `https://abc123.ngrok-free.app`). Keep ngrok running.

## Step 3: Create a GitHub App

1. Go to: **Settings** → **Developer settings** → **GitHub Apps** → **New GitHub App**
2. **App name**: `githook-local`
3. **Homepage URL**: `https://<your-ngrok-url>`
4. **Webhook URL**: `https://<your-ngrok-url>/webhooks/github`
5. **Webhook secret**: `devsecret` (for testing)
6. **Callback URL**: `https://<your-ngrok-url>/auth/github/callback`
   - Required if enabling "Request user authorization (OAuth)"
   - Path must be `/auth/github/callback`
7. **Permissions**: Repository metadata (read), Pull requests (read & write)
8. **Subscribe to events**: Pull request, Push, Check suite
9. Create the app and download the private key

## Step 4: Configure githook

Edit `config.yaml` and keep only the core sections:

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

> Provider onboarding and webhook configuration are handled via the Connect APIs; the open-source config only defines server/storage/auth.

## Step 5: Start the Server

```bash
go run ./main.go serve --config config.yaml
```

## Step 6: Create the Provider Instance

Create a provider config file (YAML):

```yaml
redirect_base_url: https://app.example.com/oauth/complete
webhook:
  secret: devsecret
app:
  app_id: 12345
  private_key_path: ./github.pem
oauth:
  client_id: your-client-id
  client_secret: your-client-secret
```

Create the provider instance:

```bash
githook --endpoint http://localhost:8080 providers set --provider github --config-file github.yaml
```

## Step 7: Start a Worker

```bash
go run ./example/github/worker/main.go --rule-id RULE_ID --endpoint=https://<your-ngrok-url>
```

## Step 8: Install the GitHub App

Get the provider instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider github
```

Visit the OAuth installation URL:
```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

Follow the GitHub authorization flow to complete installation.

## Step 9: Trigger Events

Create a pull request or push a commit to an installed repository. The worker will receive and process the events.

## Troubleshooting

- **Webhooks not received**: Check ngrok is still running, verify `endpoint` matches ngrok URL
- **404 on callback**: Callback URL must be `/auth/github/callback`
- **Missing signature**: Webhook secret doesn't match configuration
- **No matching rules**: Stored rules don't match the webhook payload (use the CLI to create/update rules)
- **Connection refused**: Ensure Docker Compose is running
- **Database errors**: Check PostgreSQL is running and connection string is correct

## GitHub Enterprise

To use GitHub Enterprise, set the GitHub API base URL in the provider instance config:

```yaml
api:
  base_url: https://ghe.company.com/api/v3
  web_base_url: https://ghe.company.com
```

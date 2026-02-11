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

Edit `config.yaml`:

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
      app_id: YOUR_APP_ID
      private_key_path: /path/to/github.pem
      app_slug: your-app-slug
    api:
      base_url: https://api.github.com
    oauth:
      client_id: your-oauth-client-id
      client_secret: your-oauth-client-secret

watermill:
  driver: amqp
  amqp:
    url: amqp://guest:guest@localhost:5672/
    mode: durable_queue

storage:
  driver: postgres
  dsn: postgres://githook:githook@localhost:5432/githook?sslmode=disable
  dialect: postgres
  auto_migrate: true

oauth:
  redirect_base_url: https://app.example.com/success

rules:
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready
  - when: action == "closed" && pull_request.merged == true
    emit: pr.merged
  - when: head_commit.id != "" && commits[0].id != "" && commits[1] == null
    emit: github.commit.created
  - when: action == "requested" && check_suite.head_commit.id != ""
    emit: github.commit.created
```

## Step 5: Start the Server

```bash
go run ./main.go serve --config config.yaml
```

## Step 6: Start a Worker

```bash
go run ./example/github/worker/main.go --config config.yaml --driver amqp
```

## Step 7: Install the GitHub App

Get the provider instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider github
```

Visit the OAuth installation URL:
```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

Follow the GitHub authorization flow to complete installation.

## Step 8: Trigger Events

Create a pull request or push a commit to an installed repository. The worker will receive and process the events.

## Troubleshooting

- **Webhooks not received**: Check ngrok is still running, verify `public_base_url` matches ngrok URL
- **404 on callback**: Callback URL must be `/auth/github/callback`
- **Missing signature**: Webhook secret doesn't match configuration
- **No matching rules**: Rules in config don't match the webhook payload
- **Connection refused**: Ensure Docker Compose is running
- **Database errors**: Check PostgreSQL is running and connection string is correct

## Multiple Provider Instances

For GitHub.com + GitHub Enterprise setups, configure multiple instances:

```yaml
providers:
  github:
    enabled: true
    api:
      base_url: https://api.github.com
    # ... other config

  github_enterprise:
    enabled: true
    api:
      base_url: https://ghe.company.com/api/v3
    # ... other config
```

Get instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider github
```

Use with OAuth:
```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

# Getting Started: GitHub

Set up a GitHub webhook pipeline: run the server, create a provider instance, install the GitHub App, and receive events.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- ngrok (for local dev): https://ngrok.com/download
- A GitHub account

## 1) Start dependencies

```bash
docker compose up -d
```

## 2) Expose with ngrok (local only)

```bash
ngrok http 8080
```

Copy the HTTPS forwarding URL (e.g., `https://abc123.ngrok-free.app`). Keep ngrok running.

## 3) Create a GitHub App

1. **Settings** → **Developer settings** → **GitHub Apps** → **New GitHub App**
2. **Homepage URL**: `https://<your-ngrok-url>`
3. **Webhook URL**: `https://<your-ngrok-url>/webhooks/github`
4. **Webhook secret**: `devsecret`
5. **Callback URL**: `https://<your-ngrok-url>/auth/github/callback`
6. **Permissions**: Repository metadata (read), Pull requests (read & write)
7. **Subscribe to events**: Pull request, Push, Check suite
8. Create the app and download the private key

## 4) Configure the server

`config.yaml`:

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

# Used when a provider instance does not set its own redirect_base_url
redirect_base_url: https://app.example.com/success
```

Start the server:

```bash
go run ./main.go serve --config config.yaml
```

## 5) Create the provider instance

Create `github.yaml`:

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

Create the instance:

```bash
githook --endpoint http://localhost:8080 providers create \
  --provider github \
  --config-file github.yaml
```

You can override the redirect URL with `--redirect-base-url` if needed.

## 6) Create a driver + rule

```bash
githook --endpoint http://localhost:8080 drivers create --name amqp --config-file amqp.yaml

githook --endpoint http://localhost:8080 rules create \
  --when 'action == "opened"' \
  --emit pr.opened \
  --driver-id <driver-id>
```

## 7) Install the GitHub App

Get the provider instance hash:

```bash
githook --endpoint http://localhost:8080 providers list --provider github
```

Open the install URL:

```
http://localhost:8080/?provider=github&instance=<instance-hash>
```

Complete the GitHub authorization flow.

## 8) Trigger events

Create a pull request or push a commit to an installed repo. Your worker should receive events.

## GitHub Enterprise

Use your enterprise base URLs in the provider config:

```yaml
api:
  base_url: https://ghe.company.com/api/v3
  web_base_url: https://ghe.company.com
```

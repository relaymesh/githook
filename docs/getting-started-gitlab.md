# Getting Started: GitLab

Set up a GitLab webhook pipeline: run the server, create a provider instance, authorize OAuth, and receive events.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- ngrok (for local dev): https://ngrok.com/download
- A GitLab account

## 1) Start dependencies

```bash
docker compose up -d
```

## 2) Expose with ngrok (local only)

```bash
ngrok http 8080
```

Copy the HTTPS forwarding URL (e.g., `https://abc123.ngrok-free.app`). Keep ngrok running.

## 3) Create a GitLab OAuth application

1. **User Settings** → **Applications**: https://gitlab.com/-/user_settings/applications
2. **Name**: `githook-local`
3. **Redirect URI**: `https://<your-ngrok-url>/auth/gitlab/callback`
4. **Scopes**: `read_api`, `read_repository`
5. Save and copy the **Application ID** and **Secret**

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

Create `gitlab.yaml`:

```yaml
redirect_base_url: https://app.example.com/oauth/complete
webhook:
  secret: devsecret
oauth:
  client_id: your-client-id
  client_secret: your-client-secret
```

Create the instance:

```bash
githook --endpoint http://localhost:8080 providers create \
  --provider gitlab \
  --config-file gitlab.yaml
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

`--driver-id` is the driver record ID (see `githook drivers list`).

Optional updates:

```bash
githook --endpoint http://localhost:8080 drivers update --name amqp --config-file amqp.yaml
githook --endpoint http://localhost:8080 providers delete --provider gitlab --hash <instance-hash>
```

## 7) Complete OAuth onboarding

Get the provider instance hash:

```bash
githook --endpoint http://localhost:8080 providers list --provider gitlab
```

Open the OAuth URL:

```
http://localhost:8080/?provider=gitlab&instance=<instance-hash>
```

Authorize access.

## 8) Configure webhooks

GitLab requires webhooks per project or group.

- **URL**: `https://<your-ngrok-url>/webhooks/gitlab`
- **Secret token**: `devsecret`

## 9) Trigger events

Create a merge request or push a commit to a repo with the webhook enabled.

## Self-hosted GitLab

```yaml
api:
  base_url: https://gitlab.company.com/api/v4
  web_base_url: https://gitlab.company.com
```

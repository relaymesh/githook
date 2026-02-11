# Getting Started: GitLab

Build a working GitLab webhook pipeline: start the broker stack, run the server, run a worker, and connect GitLab via OAuth.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- ngrok (for local development - [download here](https://ngrok.com/download))
- A GitLab account

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

## Step 3: Create a GitLab OAuth Application

1. Go to: **GitLab User Settings** → **Applications**: https://gitlab.com/-/user_settings/applications
2. **Name**: `githook-local`
3. **Redirect URI**: `https://<your-ngrok-url>/auth/gitlab/callback`
   - Path must be `/auth/gitlab/callback`
4. **Scopes**: `read_api`, `read_repository`
5. Save application and copy the **Application ID** and **Secret**

## Step 4: Configure githook

Edit `config.yaml`:

```yaml
server:
  port: 8080
  public_base_url: https://<your-ngrok-url>

providers:
  gitlab:
    enabled: true
    webhook:
      secret: devsecret
    api:
      base_url: https://gitlab.com/api/v4
      web_base_url: https://gitlab.com
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
  - when: object_kind == "merge_request" && object_attributes.state == "opened"
    emit: gitlab.mr.opened
  - when: object_kind == "push" && project.name == "test"
    emit: gitlab.push
```

## Step 5: Start the Server

```bash
go run ./main.go serve --config config.yaml
```

## Step 6: Start a Worker

```bash
go run ./example/gitlab/worker/main.go --config config.yaml --driver amqp
```

## Step 7: Complete OAuth Onboarding

Get the provider instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider gitlab
```

Visit the OAuth installation URL:
```
http://localhost:8080/?provider=gitlab&instance=<instance-hash>
```

Authorize access to your GitLab account.

## Step 8: Configure Webhook

1. Open your GitLab project
2. Go to **Settings** → **Webhooks**
3. **URL**: `https://<your-ngrok-url>/webhooks/gitlab`
4. **Secret token**: `devsecret`
5. **Trigger**: Merge request events, Push events
6. Add webhook

## Step 9: Trigger Events

Create a merge request or push a commit. The worker will receive and process the events.

## Troubleshooting

- **Webhooks not received**: Check ngrok is running, verify URL matches
- **401 unauthorized**: OAuth credentials incorrect
- **Callback failed**: Callback URL must be `/auth/gitlab/callback`
- **Connection refused**: Ensure Docker Compose is running

## Self-Hosted GitLab

For self-hosted GitLab instances:

```yaml
providers:
  gitlab:
    api:
      base_url: https://gitlab.company.com/api/v4
      web_base_url: https://gitlab.company.com
```

Configure OAuth application in your self-hosted instance.

# Getting Started: Bitbucket

Build a working Bitbucket webhook pipeline: start the broker stack, run the server, run a worker, and connect Bitbucket via OAuth.

## Prerequisites

- Go 1.24+
- Docker + Docker Compose
- ngrok (for local development - [download here](https://ngrok.com/download))
- A Bitbucket account

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

## Step 3: Create a Bitbucket OAuth Consumer

1. Go to: **Workspace Settings** → **OAuth consumers** → **Add consumer**
2. **Name**: `githook-local`
3. **Callback URL**: `https://<your-ngrok-url>/auth/bitbucket/callback`
   - Path must be `/auth/bitbucket/callback`
4. **Permissions**: Check `repository` (read/write)
5. Save consumer and copy the **Key** and **Secret**

## Step 4: Configure githook

Edit `config.yaml`:

```yaml
server:
  port: 8080
  public_base_url: https://<your-ngrok-url>

providers:
  bitbucket:
    enabled: true
    webhook:
      secret: devsecret  # Optional
    api:
      base_url: https://api.bitbucket.org/2.0
      web_base_url: https://bitbucket.org
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
  - when: pullrequest.state == "OPEN"
    emit: bitbucket.pr.opened
  - when: push.changes != null
    emit: bitbucket.push
```

## Step 5: Start the Server

```bash
go run ./main.go serve --config config.yaml
```

## Step 6: Start a Worker

```bash
go run ./example/bitbucket/worker/main.go --config config.yaml --driver amqp
```

## Step 7: Complete OAuth Onboarding

Get the provider instance hash:
```bash
githook --endpoint http://localhost:8080 providers list --provider bitbucket
```

Visit the OAuth installation URL:
```
http://localhost:8080/?provider=bitbucket&instance=<instance-hash>
```

Authorize access to your Bitbucket workspace.

## Step 8: Configure Webhooks

Bitbucket allows webhooks at both repository and workspace levels.

### Repository-Level Webhook

1. Open your Bitbucket repository
2. Go to **Repository settings** → **Webhooks** → **Add webhook**
3. **Title**: `githook-local`
4. **URL**: `https://<your-ngrok-url>/webhooks/bitbucket`
5. **Status**: Active (checked)
6. **Triggers**:
   - Pull request (created, updated, approved, merged)
   - Repository push
   - Tag push (optional)
7. Click **Save**

### Workspace-Level Webhook (For All Repositories in Workspace)

1. Go to **Workspace settings** → **Webhooks** → **Add webhook**
2. **Title**: `githook-workspace`
3. **URL**: `https://<your-ngrok-url>/webhooks/bitbucket`
4. **Status**: Active (checked)
5. **Triggers**:
   - Pull request (created, updated, approved, merged)
   - Repository push
   - Tag push (optional)
6. Click **Save**

**Note:** Workspace webhooks send events for all repositories in the workspace. Use repository webhooks for granular control.

### Multiple Workspaces/Repositories

To enable webhooks across multiple workspaces or repositories, configure the webhook for each repository or workspace individually using the Bitbucket UI. Use the same webhook URL for all repositories.

## Step 9: Trigger Events

Create a pull request or push a commit. The worker will receive and process the events.

## Troubleshooting

- **Webhooks not received**: Check ngrok is running, verify URL matches
- **401 unauthorized**: OAuth credentials incorrect
- **Callback failed**: Callback URL must be `/auth/bitbucket/callback`
- **Connection refused**: Ensure Docker Compose is running

## Bitbucket Server (Self-Hosted)

For Bitbucket Server instances:

```yaml
providers:
  bitbucket:
    api:
      base_url: https://bitbucket.company.com/rest/api/2.0
      web_base_url: https://bitbucket.company.com
```

Configure OAuth consumer in your Bitbucket Server instance.

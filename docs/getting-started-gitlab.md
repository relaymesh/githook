# Getting Started: GitLab

This guide walks through running the GitLab webhook example and creating a GitLab webhook for real payloads.

## 1) Prerequisites

- Go 1.21+
- Docker + Docker Compose
- A GitLab account and a test project

## 2) Start the local brokers

From the repo root:

```bash
docker compose up -d
```

## 3) Run the server

```bash
go run ./main.go serve --config example/gitlab/app.yaml
```

## 4) Run the worker

```bash
go run ./example/gitlab/worker/main.go
```

## 5) Send a test webhook

```bash
./scripts/send_webhook.sh gitlab merge_request example/gitlab/merge_request.json
```

## 6) Create a GitLab OAuth Application

To enable GitLab integration with OAuth (for onboarding and repository access):

1. Go to **GitLab User Settings** -> **Applications**: https://gitlab.com/-/user_settings/applications
2. **Name**: `githook-local` (any name is fine)
3. **Redirect URI**: `http://localhost:8080/auth/gitlab/callback`
   - **Important**: The path must be `/auth/gitlab/callback` (not `/oauth/gitlab/callback`)
4. **Scopes**: Check the following:
   - `read_api` - Read access to the API
   - `read_repository` - Read access to repositories
5. Click **Save application**
6. Copy the **Application ID** and **Secret** - you'll need these for your config

## 7) Configure a GitLab webhook

1. Open your GitLab project.
2. Go to **Settings** -> **Webhooks**.
3. URL: `http://localhost:8080/webhooks/gitlab`
4. Secret token: choose a random string. Save it.
5. Trigger events:
   - Merge request events
   - Push events (optional)
6. Add webhook.

### Update your config

Set the webhook secret and OAuth credentials in `example/gitlab/app.yaml` or export them as env vars:

```yaml
providers:
  gitlab:
    enabled: true
    webhook:
      secret: ${GITLAB_WEBHOOK_SECRET}
    api:
      base_url: https://gitlab.com/api/v4
      web_base_url: https://gitlab.com
    oauth:
      client_id: ${GITLAB_OAUTH_CLIENT_ID}
      client_secret: ${GITLAB_OAUTH_CLIENT_SECRET}
      scopes: ["read_api"]
```

Then:

```bash
export GITLAB_WEBHOOK_SECRET="your-secret"
export GITLAB_OAUTH_CLIENT_ID="your-application-id"
export GITLAB_OAUTH_CLIENT_SECRET="your-application-secret"
go run ./main.go serve --config example/gitlab/app.yaml
```

## 8) Optional: use ngrok for remote webhooks

```bash
ngrok http 8080
```

Update your GitLab configuration with your ngrok URL:
- **OAuth Application Redirect URI**: `https://your-ngrok-url.ngrok-free.app/auth/gitlab/callback`
- **Webhook URL**: `https://your-ngrok-url.ngrok-free.app/webhooks/gitlab`

Also update your config file with the public base URL:

```yaml
server:
  public_base_url: https://your-ngrok-url.ngrok-free.app
```

## 9) Start the onboarding flow

Once everything is configured, you can start the GitLab onboarding flow by visiting:

```
http://localhost:8080/?provider=gitlab
```

Or with ngrok:

```
https://your-ngrok-url.ngrok-free.app/?provider=gitlab
```

This will redirect you to GitLab to authorize the application and sync your repositories.

### Using Multiple Provider Instances

If you have multiple GitLab provider instances configured (e.g., GitLab.com and self-hosted GitLab), you need to specify which instance to use with the `instance` parameter:

```
http://localhost:8080/?provider=gitlab&instance=<instance-key>
```

To get the instance key, run:

```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider gitlab
```

This will show all configured GitLab provider instances and their corresponding instance keys (hashes).

## 10) Troubleshooting

- `signature mismatch`: secret token does not match.
- `no matching rules`: ensure rules in `example/gitlab/app.yaml` match your payload.
- `404 page not found` on callback: verify the OAuth application redirect URI is `/auth/gitlab/callback`.
- `missing oauth config`: ensure `GITLAB_OAUTH_CLIENT_ID` and `GITLAB_OAUTH_CLIENT_SECRET` are set.

# Getting Started: Bitbucket

This guide walks through running the Bitbucket webhook example and creating a Bitbucket webhook for real payloads.

## 1) Prerequisites

- Go 1.21+
- Docker + Docker Compose
- A Bitbucket account and a test repository

## 2) Start the local brokers

From the repo root:

```bash
docker compose up -d
```

## 3) Run the server

```bash
go run ./main.go serve --config example/bitbucket/app.yaml
```

## 4) Run the worker

```bash
go run ./example/bitbucket/worker/main.go
```

## 5) Send a test webhook

```bash
./scripts/send_webhook.sh bitbucket pullrequest:created example/bitbucket/pullrequest_created.json
```

## 6) Configure a Bitbucket webhook

1. Open your Bitbucket repo.
2. Go to **Repository settings** -> **Webhooks** -> **Add webhook**.
3. Title: `githook-local`.
4. URL: `http://localhost:8080/webhooks/bitbucket`.
5. Events:
   - Pull request created
   - Repo push (optional)
6. Save the webhook.

### OAuth Callback URL (for onboarding)

If you're using OAuth for onboarding, configure your Bitbucket OAuth consumer with:
- **Callback URL**: `http://localhost:8080/auth/bitbucket/callback`
- **Important**: The path must be `/auth/bitbucket/callback` (not `/oauth/bitbucket/callback`)

Configure this at: **Workspace Settings** -> **OAuth consumers** -> **Add consumer** (or edit existing)

### Update your config

If you use the optional `X-Hook-UUID` validation, set the secret:

```yaml
providers:
  bitbucket:
    enabled: true
    webhook:
      secret: ${BITBUCKET_WEBHOOK_SECRET}
```

Then:

```bash
export BITBUCKET_WEBHOOK_SECRET="your-secret"
go run ./main.go serve --config example/bitbucket/app.yaml
```

## 7) Optional: use ngrok for remote webhooks

```bash
ngrok http 8080
```

Update your Bitbucket configuration with your ngrok URL:
- **Webhook URL**: `https://your-ngrok-url.ngrok-free.app/webhooks/bitbucket`
- **OAuth Callback URL**: `https://your-ngrok-url.ngrok-free.app/auth/bitbucket/callback`

Also update your config file with the public base URL:

```yaml
server:
  public_base_url: https://your-ngrok-url.ngrok-free.app
```

## 8) Start the onboarding flow (optional)

If using OAuth onboarding, you can start the Bitbucket onboarding flow by visiting:

```
http://localhost:8080/?provider=bitbucket
```

Or with ngrok:

```
https://your-ngrok-url.ngrok-free.app/?provider=bitbucket
```

### Using Multiple Provider Instances

If you have multiple Bitbucket provider instances configured, specify which instance to use with the `instance` parameter:

```
http://localhost:8080/?provider=bitbucket&instance=<instance-key>
```

To get the instance key, run:

```bash
go run ./main.go --endpoint http://localhost:8080 providers list --provider bitbucket
```

## 9) Troubleshooting

- `invalid hook uuid`: secret does not match `X-Hook-UUID`.
- `no matching rules`: ensure rules in `example/bitbucket/app.yaml` match your payload.
- `404 page not found` on callback: verify the OAuth consumer callback URL is `/auth/bitbucket/callback`.

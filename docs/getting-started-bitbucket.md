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
3. Title: `githooks-local`.
4. URL: `http://localhost:8080/webhooks/bitbucket`.
5. Events:
   - Pull request created
   - Repo push (optional)
6. Save the webhook.

### Update your config

If you use the optional `X-Hook-UUID` validation, set the secret:

```yaml
providers:
  bitbucket:
    enabled: true
    key: default
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

Update the Bitbucket webhook URL to the ngrok URL.

## 8) Troubleshooting

- `invalid hook uuid`: secret does not match `X-Hook-UUID`.
- `no matching rules`: ensure rules in `example/bitbucket/app.yaml` match your payload.

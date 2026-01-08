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

## 6) Configure a GitLab webhook

1. Open your GitLab project.
2. Go to **Settings** -> **Webhooks**.
3. URL: `http://localhost:8080/webhooks/gitlab`
4. Secret token: choose a random string. Save it.
5. Trigger events:
   - Merge request events
   - Push events (optional)
6. Add webhook.

### Update your config

Set the webhook secret in `example/gitlab/app.yaml` or export it as an env var:

```yaml
providers:
  gitlab:
    enabled: true
    key: default
    webhook:
      secret: ${GITLAB_WEBHOOK_SECRET}
```

Then:

```bash
export GITLAB_WEBHOOK_SECRET="your-secret"
go run ./main.go serve --config example/gitlab/app.yaml
```

## 7) Optional: use ngrok for remote webhooks

```bash
ngrok http 8080
```

Update the GitLab webhook URL to the ngrok URL.

## 8) Troubleshooting

- `signature mismatch`: secret token does not match.
- `no matching rules`: ensure rules in `example/gitlab/app.yaml` match your payload.

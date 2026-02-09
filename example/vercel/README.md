# Vercel Example

This example triggers Vercel deploy hooks for preview and production based on GitHub pull request events.

## Prerequisites
- githook running on `http://localhost:8080`
- `GITHUB_WEBHOOK_SECRET` set to the same value in your config
- `VERCEL_PREVIEW_HOOK_URL` and `VERCEL_PRODUCTION_HOOK_URL` from your Vercel project
- Optional SCM auth: `GITHUB_APP_ID`, `GITHUB_PRIVATE_KEY_PATH`
- Replace `dummy-org/dummy-repo`, `dummy-preview-branch`, and `dummy-prod-branch` in `example/vercel/app.yaml`
- Optional GitLab/Bitbucket: `GITLAB_WEBHOOK_SECRET`, `BITBUCKET_WEBHOOK_SECRET`

## Setup

Create a deploy hook in Vercel:
1. Project Settings → Git → Deploy Hooks
2. Create two hooks (preview + production) and copy the URLs

Start RabbitMQ:
```sh
docker compose -f example/vercel/docker-compose.yaml up -d
```

## Run

Start the server:
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem
go run ./main.go serve --config example/vercel/app.yaml
```

Start the worker:
```sh
go run ./example/vercel/worker/main.go --config example/vercel/app.yaml
```

Send a merged PR webhook (production):
```sh
./scripts/send_webhook.sh github pull_request example/github/pull_request_merged.json
```

Send an opened PR webhook (preview):
```sh
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

## Notes
- The worker prints intent only (no deploy logic).
- The rule file uses dummy repo/branch names so you must update them before running.
- The config includes GitLab/Bitbucket provider stubs if you want to extend the same pattern.

# Bitbucket Webhook Example

This example sends a Bitbucket pull request webhook to the local githook server.

## Prerequisites
- githook running on `http://localhost:8080`
- `BITBUCKET_WEBHOOK_SECRET` set to the same UUID in your config (optional)

## Run
```sh
export BITBUCKET_WEBHOOK_SECRET=example-uuid
./scripts/send_webhook.sh bitbucket pullrequest:created example/bitbucket/pull_request.json
```

Repo push event:
```sh
./scripts/send_webhook.sh bitbucket repo:push example/bitbucket/repo_push.json
```

Example config (rules) is in `example/bitbucket/app.yaml`.

# GitLab Webhook Example

This example sends a GitLab merge request webhook to the local githook server.

## Prerequisites
- githook running on `http://localhost:8080`
- `GITLAB_WEBHOOK_SECRET` set to the same value in your config (optional)

## Run
```sh
export GITLAB_WEBHOOK_SECRET=devsecret
./scripts/send_webhook.sh gitlab "Merge Request Hook" example/gitlab/merge_request.json
```

Push event:
```sh
./scripts/send_webhook.sh gitlab "Push Hook" example/gitlab/push.json
```

Example config (rules) is in `example/gitlab/app.yaml`.

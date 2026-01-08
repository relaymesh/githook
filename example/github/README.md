# GitHub Webhook Example

This example sends a GitHub pull_request webhook to the local githooks server.

## Prerequisites
- githooks running on `http://localhost:8080`
- `GITHUB_WEBHOOK_SECRET` set to the same value in your config
- Optional SCM auth: `GITHUB_APP_ID` and `GITHUB_PRIVATE_KEY_PATH`

## Run
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

Tag event:
```sh
./scripts/send_webhook.sh github create example/github/tag_created.json
```

Example config (rules) is in `example/github/app.yaml`.

## Alternate payload
Pass a JSON file to send a different payload:
```sh
./scripts/send_webhook.sh github pull_request /path/to/your.json
```

## Notes
- The example uses `X-Hub-Signature` (HMAC SHA-1) which is required by the current webhook parser.
- Set your rules to match the payload, for example:
```yaml
rules:
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready
```

## Worker Example
This worker subscribes to `pr.opened.ready` and `pr.merged` and runs custom logic.

```sh
go run ./example/github/worker
```

The example uses `worker.NewSCMClientProvider` to return the official GitHub SDK client
(`go-github`) without any manual client construction.

To target a single subscriber driver (when `watermill.drivers` is set), pass `-driver`:
```sh
go run ./example/github/worker -driver amqp
```

The worker reads the `watermill` section from your app config, so you can reuse the same YAML
you run the server with.

To use the Docker compose config directly:
```sh
go run ./example/github/worker --config config.yaml
```

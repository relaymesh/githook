# GitHub Webhook Example

This example sends a GitHub pull_request webhook to the local githooks server.

## Prerequisites
- githooks running on `http://localhost:8080`
- `GITHUB_WEBHOOK_SECRET` set to the same value in your config

## Run
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
./example/github/send_webhook.sh
```

## Alternate payload
Pass a JSON file to send a different payload:
```sh
./example/github/send_webhook.sh /path/to/your.json
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
This worker subscribes to `github.pull_request` and runs custom logic.

```sh
go run ./example/github/worker -config app.docker.yaml
```

The worker reads the `watermill` section from your app config, so you can reuse the same YAML
you run the server with.

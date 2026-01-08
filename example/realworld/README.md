# Real-world Example (Multiple Drivers)

This example runs the server with multiple brokers enabled and starts two workers:
- `worker-pr` for pull request topics.
- `worker-release` for tag creation topics.

## Prereqs
- Docker (to run RabbitMQ, NATS Streaming, Kafka, Postgres, and the HTTP sink)
- `GITHUB_WEBHOOK_SECRET` exported
- Optional SCM auth: `GITHUB_APP_ID`, `GITHUB_PRIVATE_KEY_PATH`

## Run

Start the brokers:
```sh
docker compose up -d
```

Start the server:
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem

go run ./main.go serve --config example/realworld/app.yaml
```

Start the workers (separate terminals):
```sh
go run ./example/realworld/worker-pr
```

```sh
go run ./example/realworld/worker-release
```

## Send sample webhooks

Pull request opened:
```sh
./example/realworld/send_webhook.sh example/realworld/pull_request_opened.json
```

Pull request merged:
```sh
./example/realworld/send_webhook.sh example/realworld/pull_request_merged.json
```

Tag created:
```sh
./example/realworld/send_tag.sh example/realworld/tag_created.json
```

## Notes
- Rules in `example/realworld/app.yaml` publish to multiple drivers.
- The worker config enables AMQP, NATS, Kafka, HTTP, and SQL. HTTP is publisher-only, so the worker will skip it.

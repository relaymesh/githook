# RiverQueue Example

This example publishes matched webhook events into RiverQueue (Postgres).

## 1) Start Postgres
```sh
docker compose -f example/riverqueue/docker-compose.yaml up -d
```

## 2) Run River migrations
River requires database migrations before jobs can be processed.

- Use the River migration tooling described in the official docs: https://riverqueue.com/docs

## 3) Run the server
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem

go run ./main.go serve --config example/riverqueue/app.yaml
```

## 4) Send a webhook
```sh
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

## 5) Verify job inserted
```sh
psql "postgres://githook:githook@localhost:5433/githook?sslmode=disable" -c "select id, kind, queue, priority, max_attempts, created_at from river_job order by id desc limit 5;"
```

## Worker
This example worker uses the Go SDK and pulls the RiverQueue driver config from the server API.
You only need to provide the driver name.

```sh
export GITHOOK_API_BASE_URL=http://localhost:8080
go run ./example/riverqueue/worker \
  -config example/riverqueue/app.yaml \
  -driver riverqueue
```

Notes:
- `example/riverqueue/app.yaml` sets `queue: my_custom_queue` and `kind: my_job` for the RiverQueue publisher.
- The worker pulls the same queue/kind from the server via Drivers API.

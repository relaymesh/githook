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
This example worker uses the River client to consume jobs for a specific queue and kind.

```sh
go run ./example/riverqueue/worker -dsn "postgres://githook:githook@localhost:5433/githook?sslmode=disable" \
  -queue my_custom_queue \
  -kind my_job \
  -max-workers 5
```

Notes:
- `example/riverqueue/app.yaml` sets `queue: my_custom_queue` and `kind: my_job` for the RiverQueue publisher.
- The worker must use the same queue/kind to consume those jobs.

# Boilerplate Worker

This folder provides a starting point for building a worker that consumes Githooks topics.

## Easy Start (SQLite + binary)

1. Build the binaries:
```sh
go build -o githooks ./main.go
go build -o githooks-worker ./boilerplate/worker
```

2. Create the SQLite directory:
```sh
mkdir -p boilerplate/worker/data
```

3. Start the server (SQLite is already enabled in `config.yaml`):
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem

./githooks serve --config boilerplate/worker/config.yaml
```

5. Start the worker:
```sh
./githooks-worker -config boilerplate/worker/config.yaml
```

## Run (Go)

1. Start the server:
```sh
export GITHUB_WEBHOOK_SECRET=devsecret
export GITHUB_APP_ID=123
export GITHUB_PRIVATE_KEY_PATH=/path/to/github.pem

go run ./main.go serve --config boilerplate/worker/config.yaml
```

2. Start the worker:
```sh
go run ./boilerplate/worker -config boilerplate/worker/config.yaml
```

3. Send a webhook (example):
```sh
./scripts/send_webhook.sh github pull_request example/github/pull_request.json
```

## Customize
- Update `boilerplate/worker/controllers/` with your handlers.
- Update `boilerplate/worker/main.go` to register handlers.
- Update `boilerplate/worker/config.yaml` with your broker config and rules.
  - The worker auto-resolves SCM clients from `providers.*` using `worker.NewSCMClientProvider`.

## Env
Copy the env file and update secrets:
```sh
cp boilerplate/worker/.env.example .env
```

## Makefile
Common commands:
```sh
make deps-up
make run-server
make run-worker
```

Notes:
- Run `make` from `boilerplate/worker/`.
- Override paths if you copied the boilerplate elsewhere (e.g., `make ROOT=. run-worker`).

## Local Dependencies
None. The boilerplate uses SQLite for both storage and the Watermill SQL pub/sub.

## Docker
Build and run the worker container:
```sh
docker build -f boilerplate/worker/Dockerfile -t githooks-worker .
docker run --rm \
  -e GITHUB_WEBHOOK_SECRET=devsecret \
  -e GITHUB_APP_ID=123 \
  -e GITHUB_PRIVATE_KEY_PATH=/secrets/github.pem \
  githooks-worker -config /app/config.yaml
```

## Helm
You can deploy a worker using the Helm chart:

```sh
helm install githooks-worker ./charts/githooks-worker \
  --set image.repository=ghcr.io/your-org/your-worker \
  --set image.tag=latest \
  --set-file configYaml=boilerplate/worker/config.yaml
```

# CLI Usage

The `githook` binary doubles as a server and a CLI for Connect RPC endpoints.

## Server

```bash
githook serve --config config.yaml
```

## Global Flags

- `--endpoint`: Base URL for Connect RPC calls (overrides `endpoint` in config)
- `--config`: Path to config file (default: `config.yaml`)

When OAuth2 auth is enabled, the CLI uses `auth.oauth2` from the config to fetch a
client-credentials token and attaches `Authorization: Bearer <token>` to requests.
If auth is enabled on the server, the CLI must be run with a config file.
The CLI reads `endpoint` from the config when `--endpoint` is not provided.

## Init

The `server` profile disables all providers by default and includes the full
provider and watermill sections. The `cli` and `worker` profiles use `endpoint`
for API calls; the worker profile includes the full watermill section, commented
out, so you can enable it per data-plane deployment.

```bash
githook init --config server.yaml --profile server
githook init --config cli.yaml --profile cli
githook init --config worker.yaml --profile worker
```

## Environment variables

- `GITHOOK_AUTH_TOKEN`: override the cached auth token
- `GITHOOK_TOKEN_CACHE`: override the token cache path

## Installations

Use `--state-id` to filter by account; omit it to list all accounts for a provider.

```bash
githook --endpoint http://localhost:8080 installations list --provider github
githook --endpoint http://localhost:8080 installations list --provider github --state-id <state-id>
githook --endpoint http://localhost:8080 installations get --provider github --installation-id <id>
```

## Namespaces

Use `--state-id` to filter by account; omit it to list or sync all accounts for a provider.

```bash
githook --endpoint http://localhost:8080 namespaces list --provider github
githook --endpoint http://localhost:8080 namespaces list --provider github --state-id <state-id>
githook --endpoint http://localhost:8080 namespaces sync --provider gitlab
githook --endpoint http://localhost:8080 namespaces sync --provider gitlab --state-id <state-id>
githook --endpoint http://localhost:8080 namespaces webhook get --provider gitlab --repo-id <repo-id>
githook --endpoint http://localhost:8080 namespaces webhook get --provider gitlab --repo-id <repo-id> --state-id <state-id>
githook --endpoint http://localhost:8080 namespaces webhook set --provider gitlab --repo-id <repo-id> --enabled
githook --endpoint http://localhost:8080 namespaces webhook set --provider gitlab --repo-id <repo-id> --enabled --state-id <state-id>
```

## Rules

```bash
githook --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml
githook --endpoint http://localhost:8080 rules list
githook --endpoint http://localhost:8080 rules get --id <rule-id>
githook --endpoint http://localhost:8080 rules create --when 'action == "opened"' --emit pr.opened.ready
githook --endpoint http://localhost:8080 rules update --id <rule-id> --when 'action == "closed"' --emit pr.merged
githook --endpoint http://localhost:8080 rules delete --id <rule-id>
```

## Providers

```bash
githook --endpoint http://localhost:8080 providers list
githook --endpoint http://localhost:8080 providers get --provider github --hash <instance-hash>
githook --endpoint http://localhost:8080 providers set --provider github --config-file github.json
githook --endpoint http://localhost:8080 providers delete --provider github --hash <instance-hash>
```

When creating a provider instance with `providers set`, the server always generates the instance hash. The response includes the generated hash, which you must pass to `providers get`/`providers delete` and `instance=` query parameters.

## Drivers

```bash
githook --endpoint http://localhost:8080 drivers list
githook --endpoint http://localhost:8080 drivers get --name amqp
githook --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.json
githook --endpoint http://localhost:8080 drivers delete --name amqp
```

## Rules (curl)

Connect RPC endpoints accept JSON payloads over HTTP.

```bash
curl -X POST http://localhost:8080/cloud.v1.RulesService/ListRules \
  -H 'Content-Type: application/json' \
  -d '{}'

curl -X POST http://localhost:8080/cloud.v1.RulesService/GetRule \
  -H 'Content-Type: application/json' \
  -d '{"id":"<rule-id>"}'

curl -X POST http://localhost:8080/cloud.v1.RulesService/CreateRule \
  -H 'Content-Type: application/json' \
  -d '{"rule":{"when":"action == \"opened\"","emit":["pr.opened.ready"],"drivers":["amqp"]}}'

curl -X POST http://localhost:8080/cloud.v1.RulesService/UpdateRule \
  -H 'Content-Type: application/json' \
  -d '{"id":"<rule-id>","rule":{"when":"action == \"closed\"","emit":["pr.merged"]}}'

curl -X POST http://localhost:8080/cloud.v1.RulesService/DeleteRule \
  -H 'Content-Type: application/json' \
  -d '{"id":"<rule-id>"}'
```

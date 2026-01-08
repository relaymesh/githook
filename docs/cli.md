# CLI Usage

The `githooks` binary doubles as a server and a CLI for Connect RPC endpoints.

## Server

```bash
githooks serve --config config.yaml
```

## Global Flags

- `--endpoint`: Base URL for Connect RPC calls (overrides `endpoint` in config)
- `--config`: Path to config file (default: `config.yaml`)

When OAuth2 auth is enabled, the CLI uses `auth.oauth2` from the config to fetch a
client-credentials token and attaches `Authorization: Bearer <token>` to requests.
If auth is enabled on the server, the CLI must be run with a config file.
The CLI reads `endpoint` from the config when `--endpoint` is not provided.

## Auth

```bash
githooks auth
```

`auth` starts a local callback listener (loopback), prints the login URL, and attempts to open it in a browser.

## Init

```bash
githooks init --config config.yaml
```

## Environment variables

- `GITHOOKS_AUTH_TOKEN`: override the cached auth token
- `GITHOOKS_TOKEN_CACHE`: override the token cache path

## Installations

```bash
githooks --endpoint http://localhost:8080 installations list --state-id <state-id>
githooks --endpoint http://localhost:8080 installations get --provider github --installation-id <id>
```

## Namespaces

```bash
githooks --endpoint http://localhost:8080 namespaces list --state-id <state-id>
githooks --endpoint http://localhost:8080 namespaces sync --state-id <state-id> --provider gitlab
githooks --endpoint http://localhost:8080 namespaces webhook get --state-id <state-id> --provider gitlab --repo-id <repo-id>
githooks --endpoint http://localhost:8080 namespaces webhook set --state-id <state-id> --provider gitlab --repo-id <repo-id> --enabled
```

## Rules

```bash
githooks --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml
githooks --endpoint http://localhost:8080 rules list
githooks --endpoint http://localhost:8080 rules get --id <rule-id>
githooks --endpoint http://localhost:8080 rules create --when 'action == "opened"' --emit pr.opened.ready
githooks --endpoint http://localhost:8080 rules update --id <rule-id> --when 'action == "closed"' --emit pr.merged
githooks --endpoint http://localhost:8080 rules delete --id <rule-id>
```

## Providers

```bash
githooks --endpoint http://localhost:8080 providers list
githooks --endpoint http://localhost:8080 providers get --provider github --key default
githooks --endpoint http://localhost:8080 providers set --provider github --key default --config-file github.json
githooks --endpoint http://localhost:8080 providers delete --provider github --key default
```

## Drivers

```bash
githooks --endpoint http://localhost:8080 drivers list
githooks --endpoint http://localhost:8080 drivers get --name amqp
githooks --endpoint http://localhost:8080 drivers set --name amqp --config-file amqp.json
githooks --endpoint http://localhost:8080 drivers delete --name amqp
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

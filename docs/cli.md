# CLI Usage

The `githook` binary doubles as a server and a CLI for Connect RPC endpoints.

## Server

```bash
githook serve --config config.yaml
```

## Global Flags

- `--endpoint`: Base URL for Connect RPC calls (overrides `endpoint` in config)
- `--config`: Path to config file (default: `config.yaml`)
- `--tenant-id`: Tenant ID to send in `X-Tenant-ID` (default: `default`) when hitting the API

When OAuth2 auth is enabled, the CLI uses `auth.oauth2` from the config to fetch a
client-credentials token and attaches `Authorization: Bearer <token>` to requests.
If auth is enabled on the server, the CLI must be run with a config file.
The CLI reads `endpoint` from the config when `--endpoint` is not provided.

## Init

The `server` profile includes the full provider and watermill sections. The `cli`
and `worker` profiles use `endpoint`
for API calls; the worker profile includes the full watermill section, commented
out, so you can enable it per data-plane deployment.

```bash
githook init --config server.yaml --profile server
githook init --config cli.yaml --profile cli
githook init --config worker.yaml --profile worker
```

## Environment variables

- `GITHOOK_API_KEY`: send `x-api-key` if your server expects API-key auth
- `GITHOOK_AUTH_TOKEN`: override the cached auth token
- `GITHOOK_TOKEN_CACHE`: override the token cache path

## Installations

Use `--state-id` to filter by account; omit it to list all accounts for a provider. Add `--tenant-id` when you need to scope the CLI to a specific tenant (default: `default`).

```bash
githook --endpoint http://localhost:8080 installations list --provider github
githook --endpoint http://localhost:8080 installations list --provider github --state-id <state-id>
githook --endpoint http://localhost:8080 installations get --provider github --installation-id <id>
```

## Namespaces

Use `--state-id` to filter by account; omit it to list or sync all accounts for a provider. Use `--tenant-id` to scope the request to a specific tenant (default: `default`).

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

Throw `--tenant-id` on these commands when targeting a different tenant (default `default`).

## Providers

```bash
githook --endpoint http://localhost:8080 providers list
githook --endpoint http://localhost:8080 providers get --provider github --hash <instance-hash>
githook --endpoint http://localhost:8080 --tenant-id acme providers set --provider github --config-file github.yaml
githook --endpoint http://localhost:8080 providers delete --provider github --hash <instance-hash>
```

`providers set` reads the provided YAML and stores it as the provider instance configuration. Because the CLI automatically sends `X-Tenant-ID`, you can also override the tenant via `--tenant-id` so the new provider lands in the right workspace. The API generates the instance hash on creation; store it for later use with `providers get`, `providers delete`, and the OAuth onboarding `instance=` query parameter.

### Config File Format

The config file can include `redirect_base_url` for per-instance OAuth redirects:

```yaml
redirect_base_url: https://app.example.com/oauth/complete
app:
  app_id: 12345
  private_key_path: ./github.pem
webhook:
  secret: your-webhook-secret
oauth:
  client_id: your-client-id
  client_secret: your-client-secret
```

### Flags for `providers set`

| Flag | Description |
|------|-------------|
| `--provider` | Provider name: `github`, `gitlab`, or `bitbucket` |
| `--config-file` | Path to provider YAML configuration file (can include `redirect_base_url`) |
| `--enabled` | Enable this provider instance (default: `true`) |
| `--redirect-base-url` | Per-instance OAuth redirect URL (overrides value in config file) |

## Drivers

```bash
githook --endpoint http://localhost:8080 drivers list
githook --endpoint http://localhost:8080 drivers get --name amqp
githook --endpoint http://localhost:8080 --tenant-id acme drivers set --name amqp --config-file amqp.yaml
githook --endpoint http://localhost:8080 drivers delete --name amqp
```

The driver config file must be YAML and the CLI converts it to the JSON payload required by the API. `--tenant-id` decides which tenant owns the driver.

## Rules (curl)

Connect RPC endpoints accept JSON payloads over HTTP. Include `X-Tenant-ID` when targeting a specific tenant (default: `default`).

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

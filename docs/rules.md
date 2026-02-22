# Rules Engine

Rules use JSONPath for field access and boolean logic for matching. Rules are stored on the server and managed via the CLI/API (not `config.yaml`).

## Managing Rules
```sh
githook --endpoint http://localhost:8080 rules list
githook --endpoint http://localhost:8080 rules create \
  --when 'action == "opened" && pull_request.draft == false' \
  --emit pr.opened.ready \
  --driver-id <driver-id>
```
`--driver-id` should match the driver record ID (see `githook drivers list`).

## Rule Shape
```yaml
when: action == "opened" && pull_request.draft == false
emit: pr.opened.ready
driver_id: <driver-id>
```

To test locally without changing stored rules, use `rules match` with a YAML file:
```sh
githook --endpoint http://localhost:8080 rules match --payload-file payload.json --rules-file rules.yaml
```

```yaml
# rules.yaml
rules:
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready
    driver_id: <driver-id>
```

## JSONPath
- Bare identifiers are treated as root JSONPath (e.g., `action` becomes `$.action`).
- Arrays are supported: `$.pull_request.commits[0].created == true`.

## Functions
- `contains(value, needle)` works for strings, arrays, and maps.
  - Example: `contains(labels, "bug")`
- `like(value, pattern)` matches SQL-like patterns (`%` for any length, `_` for one char).
  - Example: `like(ref, "refs/heads/%")`

### Nested Examples
```yaml
when: contains($.pull_request.labels[*].name, "bug")
emit: pr.label.bug
driver_id: <driver-id>
```

## Driver Targeting
Each rule targets a single driver via `driver_id`. To publish the same event to multiple drivers, create multiple rules with the same `when`/`emit` and different `driver_id` values.

## Fan-Out Topics
Use a list for `emit` to publish the same event to multiple topics.

## Strict Mode
Strict mode is a server setting; use `githook rules match --strict` to test locally.

## System Rules (GitHub App)
GitHub App installation events are always processed to keep `githook_installations` in sync.
These updates are applied even if no user rule matches and cannot be disabled by rules:

- `installation`
- `installation_repositories`

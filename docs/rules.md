# Rules

Rules decide which webhook events get published. Rules are stored on the server and managed via the CLI/API (not `config.yaml`).

## Manage rules

```bash
githook --endpoint http://localhost:8080 rules list

githook --endpoint http://localhost:8080 rules create \
  --when 'action == "opened" && pull_request.draft == false' \
  --emit pr.opened.ready \
  --driver-id <driver-id>
```

`--driver-id` must be a driver record ID (see `githook drivers list`).

## Rule shape

```yaml
when: action == "opened" && pull_request.draft == false
emit: pr.opened.ready
driver_id: <driver-id>
```

`emit` can be a string or list:

```yaml
emit:
  - pr.opened
  - audit.pr.opened
```

## Expression basics

- Expressions use boolean logic (`&&`, `||`, `!`) and comparisons (`==`, `!=`, `<`, `>`).
- Strings use single or double quotes.
- `true`, `false`, and `null` are supported.

## JSONPath access

Fields are resolved using JSONPath:

- Bare identifiers are treated as root paths (e.g., `action` → `$.action`).
- Explicit JSONPath works as-is (e.g., `$.pull_request.title`).
- Arrays are supported (e.g., `$.pull_request.labels[*].name`).

Examples:

```yaml
when: contains($.pull_request.labels[*].name, "bug")
emit: pr.label.bug
driver_id: <driver-id>
```

## Functions

- `contains(value, needle)` for strings, arrays, and maps
- `like(value, pattern)` where `%` matches any length and `_` matches one character

Examples:

```yaml
when: contains(labels, "bug") && like(ref, "refs/heads/%")
emit: pr.opened.ready
driver_id: <driver-id>
```

## Testing rules locally

Use `rules match` without storing rules:

```bash
githook --endpoint http://localhost:8080 rules match \
  --payload-file payload.json \
  --rules-file rules.yaml
```

```yaml
# rules.yaml
rules:
  - when: action == "opened"
    emit: pr.opened
    driver_id: <driver-id>
```

## Strict mode

When strict mode is enabled (`rules_strict` in server config), any rule that references a missing field is skipped. You can test strict mode locally with:

```bash
githook --endpoint http://localhost:8080 rules match --strict --payload-file payload.json --rules-file rules.yaml
```

## Driver targeting

Each rule targets one driver via `driver_id`. To publish to multiple drivers, create multiple rules with the same `when`/`emit` and different `driver_id` values.

## System rules (GitHub App)

GitHub App installation events are always processed to keep installation data in sync (even if no user rule matches):

- `installation`
- `installation_repositories`

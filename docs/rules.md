# Rules Engine

Rules use JSONPath for field access and boolean logic for matching. Each matching rule emits a topic and can optionally target specific drivers.

## Syntax
```yaml
rules:
  - when: action == "opened" && pull_request.draft == false
    emit: pr.opened.ready
  - when: action == "closed" && pull_request.merged == true
    emit: pr.merged
    drivers: [amqp, http]
  - when: action == "closed" && pull_request.merged == true
    emit: [pr.merged, audit.pr.merged]
  - when: contains(labels, "bug")
    emit: issue.label.bug
  - when: like(ref, "refs/heads/%")
    emit: push.branch
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
rules:
  - when: contains($.pull_request.labels[*].name, "bug")
    emit: pr.label.bug

  - when: contains($.repository.full_name, "acme/")
    emit: repo.acme

  - when: like($.pull_request.head.ref, "release/%")
    emit: pr.branch.release

  - when: like($.sender.login, "bot-%")
    emit: sender.bot
```

## Driver Targeting
- `drivers` omitted: publish to all configured drivers.
- `drivers` specified: publish only to those drivers.

## Fan-Out Topics
Use a list for `emit` to publish the same event to multiple topics.

## Strict Mode
Set `rules_strict: true` to skip a rule if any JSONPath in its `when` clause is missing.

## System Rules (GitHub App)
GitHub App installation events are always processed to keep `githooks_installations` in sync.
These updates are applied even if no user rule matches and cannot be disabled by rules:

- `installation`
- `installation_repositories`

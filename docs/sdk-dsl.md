# SDK DSL (Portable Worker Spec)

This document proposes a language-agnostic DSL that future SDKs can consume to bootstrap workers consistently across languages. It is **not** a runtime requirement for Go today, but it provides a standard contract for multi-language SDKs.

## Goals

- **Portable**: Works across Go, Node, Python, Rust.
- **Minimal**: Only describes worker behavior and handler routing.
- **Consistent**: Mirrors the existing event envelope and rules engine.
- **Composable**: Lets SDKs generate skeletons or wire handlers by name.

## Canonical Event Envelope

The canonical schema is `idl/cloud/v1/githook.proto`. Broker messages are encoded
as `cloud.v1.EventPayload` (protobuf), and the raw webhook JSON is preserved in
`payload`.

```proto
message EventPayload {
  string provider = 1;
  string name = 2;
  bytes payload = 3;
}
```

For non‑Go SDKs, you can decode the protobuf envelope directly. If you need a JSON
bridge, the JSON envelope should mirror the fields above plus metadata:

```json
{
  "provider": "github",
  "name": "pull_request",
  "topic": "pr.merged",
  "payload": { "...": "raw webhook JSON" },
  "metadata": {
    "request_id": "…",
    "installation_id": "…"
  }
}
```

## Worker DSL (Proposed)

```yaml
version: v1
worker:
  name: pr-worker
  topics: ["pr.opened.ready", "pr.merged"]
  concurrency: 5
  retry:
    max_attempts: 3
    backoff_ms: 1000
  middleware:
    - logging
    - metrics
handlers:
  pr.opened.ready: onPullRequestReady
  pr.merged: onPullRequestMerged
clients:
  github: true
  gitlab: true
  bitbucket: true
```

## Rule DSL (Already Supported)

Rules are stored on the server and managed via the CLI/API:

```yaml
when: action == "closed" && pull_request.merged == true
emit: pr.merged
driver_id: <driver-id>
```

`when` supports JSONPath expressions, boolean logic, and helper functions like `contains()` and `like()`.

## SDK Contract (Future)

SDKs in other languages can:

1. Parse the Worker DSL to wire handlers.
2. Consume the event envelope (topic + provider + payload + metadata).
3. Construct provider clients using the server Installations API.

This keeps the runtime consistent while allowing each language to use idiomatic APIs.

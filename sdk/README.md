# SDKs

This directory groups language-specific SDKs that build on top of the Githooks event model and a shared, language-agnostic contract.

- `go/`: Go worker SDK (current reference implementation).

Planned:
- `js/`, `py/`, `rust/`: Thin client bindings that consume the same event envelope and rules DSL.

## Shared Contract

All SDKs are expected to:

- Consume the same event envelope (`provider`, `topic`, `payload`, `metadata`).
- Rely on the same rules DSL defined in the server config.
- Resolve provider clients via the Installations API (no direct access to platform storage; SQL subscribers are for event queues only).

## Go Worker Quick Start

Use the API-driven worker so you only need the endpoint and API key:

```go
package main

import (
  "context"
  "os"

  "githook/sdk/go/worker"
)

func main() {
  wk := worker.New(
    worker.WithEndpoint(os.Getenv("GITHOOK_ENDPOINT")),
    worker.WithAPIKey(os.Getenv("GITHOOK_API_KEY")),
    worker.WithDefaultDriver("driver-id"),
    worker.WithTopics("pr.opened.ready"),
  )

  wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
    return nil
  })

  _ = wk.Run(context.Background())
}
```

## Language Support Note

The current worker runtime depends on Watermill, which is Go‑only. That means:

- The Go SDK is the only fully supported worker runtime today.
- Other language SDKs would need their own broker consumers and runtime logic.
- We do not provide Watermill equivalents in other languages yet.

We plan to build Watermill‑like runtimes for other languages, but that work is still in the pipeline.

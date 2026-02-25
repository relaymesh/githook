# Go SDK

This SDK exposes the worker runtime under a stable path:

```go
import "githook/sdk/go/worker"
```

This is the core Go worker SDK. Use it when you want a stable SDK boundary that can mirror future language SDKs.

## Minimal example (rule id)

```go
wk := worker.New(
  worker.WithEndpoint(os.Getenv("GITHOOK_ENDPOINT")),
  worker.WithAPIKey(os.Getenv("GITHOOK_API_KEY")),
)

wk.HandleRule("rule-id", func(ctx context.Context, evt *worker.Event) error {
  // custom logic
  return nil
})

_ = wk.Run(context.Background())
```

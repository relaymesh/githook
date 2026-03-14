# Go SDK

This SDK exposes the worker runtime under a stable path:

```go
import "github.com/relaymesh/relaymesh/sdk/go/worker"
```

This is the core Go worker SDK. Use it when you want a stable SDK boundary that can mirror future language SDKs.

## Minimal example (rule id)

```go
wk := worker.New(
  worker.WithEndpoint(os.Getenv("RELAYMESH_ENDPOINT")),
  worker.WithAPIKey(os.Getenv("RELAYMESH_API_KEY")),
)

wk.HandleRule("rule-id", func(ctx context.Context, evt *worker.Event) error {
  // custom logic
  return nil
})

_ = wk.Run(context.Background())
```

## OAuth2 mode

When using OAuth2 in workers, set mode to `client_credentials`:

```go
wk := worker.New(
  worker.WithEndpoint(os.Getenv("RELAYMESH_ENDPOINT")),
  worker.WithOAuth2Config(auth.OAuth2Config{
    Enabled: true,
    Mode:    "client_credentials",
  }),
)
```

# Go SDK

This SDK exposes the worker runtime under a stable path:

```go
import "githook/sdk/go/worker"
```

This is the core Go worker SDK. Use it when you want a stable SDK boundary that can mirror future language SDKs.

## Minimal example (API-driven drivers)

```go
wk := worker.New(
  worker.WithEndpoint(os.Getenv("GITHOOK_ENDPOINT")),
  worker.WithAPIKey(os.Getenv("GITHOOK_API_KEY")),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
)

wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  // custom logic
  return nil
})

_ = wk.Run(context.Background())
```

## Driver config from API

When you want the worker to use driver configuration stored on the server (Drivers API),
pass the driver ID for each topic:

```go
wk, _ := worker.NewFromConfigPathWithDriverFromAPI("config.yaml", "")

wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  return nil
})

_ = wk.Run(context.Background())
```

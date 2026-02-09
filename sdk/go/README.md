# Go SDK

This SDK exposes the worker runtime under a stable path:

```go
import "githook/sdk/go/worker"
```

This is the core Go worker SDK. Use it when you want a stable SDK boundary that can mirror future language SDKs.

## Minimal example

```go
subCfg, _ := worker.LoadSubscriberConfig("config.yaml")
sub, _ := worker.BuildSubscriber(subCfg)

wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
)

wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
  // custom logic
  return nil
})

_ = wk.Run(context.Background())
```

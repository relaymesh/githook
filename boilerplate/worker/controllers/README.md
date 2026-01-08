# Controllers

Put handler logic here. Each function should follow:

```go
func(ctx context.Context, evt *worker.Event) error
```

Register handlers from `main.go` using `wk.HandleTopic` or `wk.HandleType`.

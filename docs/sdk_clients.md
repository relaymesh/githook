# SDK Client Injection

Use the SDK to attach client instances to each event. The worker can request SCM client credentials from the server and build provider clients locally (with a small LRU cache), or you can inject your own client resolver.

## Pattern

Use a `ClientProviderFunc` to supply your own client resolution logic:

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.ClientProviderFunc(func(ctx context.Context, evt *worker.Event) (interface{}, error) {
    // Example: return a thin client that calls your own API.
    return newSCMProxyClient(os.Getenv("SCM_PROXY_URL")), nil
  })),
)

wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  _ = evt.Client
  return nil
})
```

## Server-resolved SCM clients (recommended)

Use the remote SCM client provider to fetch credentials from the server and build
provider SDK clients locally. The provider caches up to 10 clients by default.
The server resolves enterprise vs. cloud based on the provider instance key in the event metadata.

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.NewRemoteSCMClientProvider()),
)

wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  if gh, ok := worker.GitHubClient(evt); ok {
    _, _, _ = gh.Repositories.List(ctx, "", nil)
  }
  return nil
})
```

```python
from relaymesh_githook import New, WithClientProvider, NewRemoteSCMClientProvider, GitHubClient

wk = New(
    WithClientProvider(NewRemoteSCMClientProvider()),
)

def handler(ctx, evt):
    client = GitHubClient(evt)
    if client:
        client.request_json("GET", "/user")

wk.HandleRule("rule-id", handler)
```

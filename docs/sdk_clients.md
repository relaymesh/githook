# SDK Client Injection

Workers can attach SCM clients (GitHub/GitLab/Bitbucket) to each event. The default approach is to ask the server for credentials and build clients locally with a small LRU cache.

## When to use each approach

- Remote SCM clients: best for most deployments, because the server resolves cloud vs. enterprise and handles credentials.
- Custom clients: useful when you need to route through a proxy or reuse existing auth infrastructure.

## Remote SCM clients (recommended)

The worker uses the event metadata (installation + provider instance key) to request credentials from the server. The server decides cloud vs. enterprise and returns the correct auth details.

Go:

```go
wk := worker.New(
  worker.WithEndpoint("http://localhost:8080"),
  worker.WithClientProvider(worker.NewRemoteSCMClientProvider()),
)

wk.HandleRule("<rule-id>", func(ctx context.Context, evt *worker.Event) error {
  if gh, ok := worker.GitHubClient(evt); ok {
    _, _, _ = gh.Repositories.List(ctx, "", nil)
  }
  return nil
})
```

TypeScript:

```ts
import { New, WithEndpoint, NewRemoteSCMClientProvider, GitHubClient } from "@relaymesh/githook";

const worker = New(
  WithEndpoint("http://localhost:8080"),
  NewRemoteSCMClientProvider(),
);

worker.HandleRule("<rule-id>", async (evt) => {
  const gh = GitHubClient(evt);
  if (gh) {
    await gh.requestJSON("GET", "/user");
  }
});
```

If a client cannot be resolved, the helpers return `nil`/`undefined`. Treat that as a non-fatal condition and continue handling the event.

Python:

```python
from relaymesh_githook import New, WithEndpoint, WithClientProvider, NewRemoteSCMClientProvider, GitHubClient

wk = New(
    WithEndpoint("http://localhost:8080"),
    WithClientProvider(NewRemoteSCMClientProvider()),
)

def handler(ctx, evt):
    client = GitHubClient(evt)
    if client:
        client.request_json("GET", "/user")

wk.HandleRule("<rule-id>", handler)
```

## Custom client injection

If you want full control, inject your own client resolver.

Go:

```go
wk := worker.New(
  worker.WithClientProvider(worker.ClientProviderFunc(func(ctx context.Context, evt *worker.Event) (interface{}, error) {
    return newSCMProxyClient(os.Getenv("SCM_PROXY_URL")), nil
  })),
)
```

## Event log status updates

Each SDK updates event log status automatically after your handler runs:

- Successful return/completion updates status to `success`.
- Returned errors (Go) or thrown/raised exceptions (TypeScript/Python) update status to `failed` and include the error message.

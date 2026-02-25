# SDK Client Injection

Workers can attach SCM clients (GitHub/GitLab/Bitbucket) to each event. The default approach is to ask the server for credentials and build clients locally with a small LRU cache.

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

# SDK Client Injection

Use the SDK to attach provider-specific clients to each event. You can either inject your own clients or let the SDK resolve them from the webhook payload using the `providers` config. The SDK returns a ready-to-use provider SDK client so you do not have to construct it yourself.

## Pattern

Use a `ClientProviderFunc` to supply your own client resolution logic:

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.ClientProviderFunc(func(ctx context.Context, evt *worker.Event) (interface{}, error) {
    switch evt.Provider {
    case "github":
      return newGitHubAppClient(appID, installationID, privateKeyPEM), nil
    case "gitlab":
      return newGitLabClient(token), nil
    case "bitbucket":
      return newBitbucketClient(username, appPassword), nil
    default:
      return nil, nil
    }
  })),
)

wk.HandleTopic("pr.opened.ready", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  switch evt.Provider {
  case "github":
    gh, _ := worker.GitHubClient(evt)
    _ = gh
  case "gitlab":
    gl, _ := worker.GitLabClient(evt)
    _ = gl
  case "bitbucket":
    bb, _ := worker.BitbucketClient(evt)
    _ = bb
  }
  return nil
})
```

## Auto-resolve clients from config

Use `SCMClientProvider` to resolve clients automatically from your providers config:

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.NewSCMClientProvider(cfg.Providers)),
)
```

The `providers` section in your config includes the SCM auth settings (`app.app_id`, `app.private_key_path`, OAuth client credentials, and API base URLs). For GitLab and Bitbucket, the worker resolves access tokens via the Installations API using the `installation_id` in event metadata.

By default the endpoint is resolved from `GITHOOK_ENDPOINT` (falling back to `GITHOOK_API_BASE_URL`). When neither environment variable is set it falls back to `http://localhost:8080`.

## Multi-provider worker example

This pattern uses SDK-resolved clients for GitHub, GitLab, and Bitbucket in the same handler.
See `example/vercel/worker/main.go` for a complete runnable example.

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("vercel.preview", "vercel.production"),
  worker.WithClientProvider(worker.NewSCMClientProvider(cfg.Providers)),
)

wk.HandleTopic("vercel.preview", "driver-id", func(ctx context.Context, evt *worker.Event) error {
  switch evt.Provider {
  case "github":
    gh, ok := worker.GitHubClient(evt)
    if ok {
      _ = gh
    }
  case "gitlab":
    gl, ok := worker.GitLabClient(evt)
    if ok {
      _ = gl
    }
  case "bitbucket":
    bb, ok := worker.BitbucketClient(evt)
    if ok {
      _ = bb
    }
  }
  return nil
})
```

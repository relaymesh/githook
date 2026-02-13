# SDK Client Injection

Use the SDK to attach provider-specific clients to each event. You can either inject your own clients or let the SDK resolve them from the webhook payload using the `providers` config. The SDK returns a ready-to-use provider SDK client so you do not have to construct it yourself.

## Pattern

```go
githubClient := newGitHubAppClient(appID, installationID, privateKeyPEM)
gitlabClient := newGitLabClient(token)
bitbucketClient := newBitbucketClient(username, appPassword)

wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.ProviderClients{
    GitHub: func(ctx context.Context, evt *worker.Event) (interface{}, error) { return githubClient, nil },
    GitLab: func(ctx context.Context, evt *worker.Event) (interface{}, error) { return gitlabClient, nil },
    Bitbucket: func(ctx context.Context, evt *worker.Event) (interface{}, error) { return bitbucketClient, nil },
  }),
)

wk.HandleTopic("pr.opened.ready", func(ctx context.Context, evt *worker.Event) error {
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

## Resolve tokens in workers

Use the server API to map `installation_id` → stored tokens:

```go
providerClient, err := worker.ResolveProviderClient(ctx, evt)
if err != nil {
  return err
}

switch evt.Provider {
case "github":
  gh := providerClient.(*github.Client)
  _ = gh
case "gitlab":
  gl := providerClient.(*gitlab.Client)
  _ = gl
case "bitbucket":
  bb := providerClient.(*bitbucket.Client)
  _ = bb
}
```

By default it uses `GITHOOK_API_BASE_URL`. If not set, it will read
`GITHOOK_CONFIG_PATH` (or `GITHOOK_CONFIG`) and use `server.public_base_url`
or `server.port` to build the URL. Otherwise it falls back to
`http://localhost:8080`.

This keeps webhook payloads normalized in `evt.Normalized`, while the SDK gives you the correct provider client for API calls.

## Auto-resolve clients from config

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("pr.opened.ready", "pr.merged"),
  worker.WithClientProvider(worker.NewSCMClientProvider(cfg.Providers)),
)
```

The `providers` section in your config includes the SCM auth settings (`app.app_id`, `app.private_key_path`, OAuth client credentials, and API base URLs). For GitLab and Bitbucket, the worker resolves access tokens via the Installations API using the `installation_id` in event metadata.

## Multi-provider worker example

This pattern uses SDK-resolved clients for GitHub, GitLab, and Bitbucket in the same handler.
See `example/vercel/worker/main.go` for a complete runnable example.

```go
wk := worker.New(
  worker.WithSubscriber(sub),
  worker.WithTopics("vercel.preview", "vercel.production"),
  worker.WithClientProvider(worker.NewSCMClientProvider(cfg.Providers)),
)

wk.HandleTopic("vercel.preview", func(ctx context.Context, evt *worker.Event) error {
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

# SCM Authentication

This guide covers runtime SCM authentication resolved from webhook context. GitLab and Bitbucket use OAuth tokens stored on install.

## Configuration

```yaml
providers:
  github:
    app:
      app_id: 123
      private_key_path: /secrets/github.pem
    api:
      base_url: https://api.github.com
  gitlab:
    api:
      base_url: https://gitlab.com/api/v4
  bitbucket:
    api:
      base_url: https://api.bitbucket.org/2.0
```

GitHub Enterprise: set `providers.github.api.base_url` to your API base
(for example, `https://ghe.example.com/api/v3`). The upload URL is derived automatically.

## Flow

1. Webhook arrives and is parsed (unchanged).
2. Resolve auth from the webhook payload.
3. Create an SCM client and call the provider API. The SDK returns the client instance so you do not have to construct it yourself.

## Example: webhook → auth → client

```go
resolver := auth.NewResolver(cfg.Providers)
authCtx, err := resolver.Resolve(ctx, auth.EventContext{
	Provider: evt.Provider,
	Payload:  evt.Payload,
})
if err != nil {
	return err
}

factory := scm.NewFactory(cfg.Providers)
client, err := factory.NewClient(ctx, authCtx)
if err != nil {
	return err
}

switch authCtx.Provider {
case "github":
	gh := client.(*github.Client)
	_ = gh // use gh for API calls you need
case "gitlab":
	gl := client.(*gitlab.Client)
	_ = gl
case "bitbucket":
	bb := client.(*bitbucket.Client)
	_ = bb
}
return nil
```

## Notes

- GitHub uses GitHub App authentication. Tokens are short-lived and never persisted.
- GitLab and Bitbucket use access tokens stored during OAuth install.
- Provider clients are intentionally minimal; inject your own clients if you need a full API surface.

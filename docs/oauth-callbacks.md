# OAuth Callbacks

Githooks can accept OAuth callbacks on dedicated endpoints and store installation
data in the configured SQL store. After a successful callback, Githooks redirects
to a frontend URL and appends query parameters for the provider and state.

## GitHub App install entry

GitHub App installs start from the GitHub App installation page, not from a
Githooks route. You can send users to the install URL:

```
https://github.com/apps/<app-slug>/installations/new
```

If you enable **Request user authorization (OAuth) during installation** in the
GitHub App settings, set the App **Callback URL** to:

```
https://<your-domain>/oauth/github/callback
```

This callback is separate from the webhook URL (`/webhooks/github`).

## Configuration

```yaml
server:
  public_base_url: https://app.example.com
oauth:
  redirect_base_url: https://app.example.com/oauth/complete
```

The redirect URL receives query params such as:
- `provider`
- `state`
- `installation_id` (GitHub App installs)

## Endpoints

- `/oauth/github/callback`
- `/oauth/gitlab/callback`
- `/oauth/bitbucket/callback`

## Install/Authorize entry

To start the flow, redirect users to:

```
http://localhost:8080/?provider=github
http://localhost:8080/?provider=gitlab
http://localhost:8080/?provider=bitbucket
```

To target a specific provider instance, pass `instance=<key>` (defaults to `default`):

```
http://localhost:8080/?provider=github&instance=acme-prod
```

GitHub uses the App installation URL. GitLab and Bitbucket use OAuth authorize URLs built from `providers.*` config.

## Notes

- These routes are separate from webhook endpoints to keep webhook parsing unchanged.
- GitHub App installs are initiated from GitHub, not from Githooks. The callback is only used when "Request user authorization" is enabled.
- `server.public_base_url` forces callback URLs to use your public domain instead of `localhost`.
- GitLab/Bitbucket OAuth uses the configured `providers.*.oauth.client_id` and `providers.*.oauth.client_secret`.
- GitHub App installs store the `installation_id` for later lookup.

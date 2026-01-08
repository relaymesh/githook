# API Authentication (OAuth2/OIDC)

Githooks can protect all Connect RPC endpoints with OAuth2/OIDC JWT validation.
Webhooks and SCM install callbacks remain public.

## Minimal server config

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
```

The server discovers JWKS/authorize/token endpoints from the issuer.

## CLI (machine)

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
    mode: client_credentials
    client_id: ${OAUTH_CLIENT_ID}
    client_secret: ${OAUTH_CLIENT_SECRET}
```

The CLI uses client credentials automatically and injects `Authorization: Bearer <token>`.

## Human login (browser)

```yaml
server:
  public_base_url: https://app.example.com

auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
    mode: auth_code
    client_id: ${OAUTH_CLIENT_ID}
    client_secret: ${OAUTH_CLIENT_SECRET}
    # redirect_url defaults to https://app.example.com/auth/callback
```

Endpoints:
- `GET /auth/login` (server-hosted login redirect)
- `GET /auth/callback` (server-hosted callback for auth_code)

CLI helper:
- `githooks auth` (single command for login, token fetch, and cache)
  - uses a loopback callback (`http://127.0.0.1:<port>/auth/callback`) and prints a browser URL for auth_code
  - ensure your IdP allows the loopback redirect URI (register a localhost callback)
  - opens the browser automatically when possible

## Optional fields

- `required_scopes`: scopes enforced by the server (space-separated in token).
- `jwks_url`, `authorize_url`, `token_url`: auto-discovered if omitted.
- `scopes`: defaults to `openid profile email`.

## Okta (client_credentials + auth_code)

1) Create an Authorization Server:
- Security → API → Authorization Servers → Add
- Note the issuer, e.g. `https://<your-okta-domain>/oauth2/default`

2) Create scopes (optional):
- Authorization Server → Scopes → Add `githooks.rpc`

3) Create apps:
- API Service app for machine use (client credentials)
- Web app for human login (auth code + PKCE)

4) Enable client_credentials and auth_code in policy rules.

Minimal config:

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
    required_roles: ["admin"] # optional
    required_groups: ["platform"] # optional
    required_scopes: ["githooks.rpc"]

    mode: auto
    client_id: ${OKTA_CLIENT_ID}
    client_secret: ${OKTA_CLIENT_SECRET}
    scopes: ["openid","profile","email"]
```

## Azure AD (client_credentials + auth_code)

1) Register an app:
- Azure Portal → Entra ID → App registrations → New registration
- Note Tenant ID and Application (client) ID

2) Create a client secret:
- Certificates & secrets → New client secret

3) Expose an API (audience):
- Expose an API → Set Application ID URI (e.g. `api://githooks`)

4) Add permissions/scopes if needed.

Issuer/JWKS:
- Issuer: `https://login.microsoftonline.com/<tenant-id>/v2.0`
- JWKS: `https://login.microsoftonline.com/<tenant-id>/discovery/v2.0/keys`

Minimal config:

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://login.microsoftonline.com/<tenant-id>/v2.0
    audience: api://githooks
    mode: auto
    client_id: ${AZURE_CLIENT_ID}
    client_secret: ${AZURE_CLIENT_SECRET}
```

## Google (service account / JWT access tokens)

Google doesn’t support standard client_credentials for most APIs. Use a service account:

1) Create a service account:
- Google Cloud Console → IAM & Admin → Service Accounts
- Create key (JSON)

2) Generate access tokens using the service account JWT flow.

Issuer/JWKS:
- Issuer: `https://accounts.google.com`
- JWKS: `https://www.googleapis.com/oauth2/v3/certs`

Minimal config (JWT verification on server):

```yaml
auth:
  oauth2:
    enabled: true
    issuer: https://accounts.google.com
    audience: <your-google-client-id>
```

For machine token retrieval, use a service account exchange in your client code (outside githooks).

## Multi-instance deployments

You can run separate servers for GitHub, GitLab, and Bitbucket against the same database and auth config.
Each instance should enable only its provider(s) in `providers.*.enabled`.

When multiple providers are enabled, list APIs return results across those providers. When a provider
is disabled, RPC calls for that provider return a `provider not enabled` error.

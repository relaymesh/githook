# OAuth Callbacks

githook handles OAuth callbacks to authorize and connect Git provider accounts. After successful authorization, users are redirected to your application with installation metadata.

## OAuth Flow Overview

```
User → githook OAuth URL → Provider (GitHub/GitLab/Bitbucket) → Callback → Store Token → Redirect
```

## Callback Endpoints

Configure these callback URLs in your provider OAuth applications:

- **GitHub**: `https://your-domain.com/auth/github/callback`
- **GitLab**: `https://your-domain.com/auth/gitlab/callback`
- **Bitbucket**: `https://your-domain.com/auth/bitbucket/callback`

**Important**: The path must be `/auth/{provider}/callback` (not `/oauth/{provider}/callback`)

## Configuration

```yaml
endpoint: https://your-domain.com  # Your public URL

redirect_base_url: https://app.example.com/oauth/complete  # Where to send users after OAuth
```

## Initiating OAuth

Redirect users to:

```
https://your-domain.com/?provider=<provider>&instance=<instance-hash>
```

**Get instance hash:**
```bash
githook --endpoint https://your-domain.com providers list --provider github
```

**Examples:**
- GitHub: `https://your-domain.com/?provider=github&instance=a1b2c3d4`
- GitLab: `https://your-domain.com/?provider=gitlab&instance=a1b2c3d4`
- Bitbucket: `https://your-domain.com/?provider=bitbucket&instance=a1b2c3d4`

## What Happens

1. User clicks the OAuth URL
2. Redirected to provider's authorization page
3. User authorizes the application
4. Provider redirects to githook callback (`/auth/{provider}/callback`)
5. githook exchanges authorization code for access token
6. Token stored in PostgreSQL
7. User redirected to `redirect_base_url` with query parameters:
   - `provider` - The provider name
   - `state` - CSRF token
   - `installation_id` - GitHub installation ID (GitHub only)

## Provider-Specific Notes

### GitHub

**GitHub App Installation:**
- Installations start from GitHub App settings, not githook
- Direct URL: `https://github.com/apps/<app-slug>/installations/new`
- Webhook URL: `https://your-domain.com/webhooks/github` (separate from callback)

**OAuth (Optional):**
- Only required if "Request user authorization (OAuth)" is enabled in GitHub App settings
- Callback URL: `https://your-domain.com/auth/github/callback`

### GitLab

- OAuth required for all GitLab integrations
- Uses `providers.gitlab.oauth.client_id` and `client_secret`
- Callback URL: `https://your-domain.com/auth/gitlab/callback`

### Bitbucket

- OAuth required for all Bitbucket integrations
- Uses `providers.bitbucket.oauth.client_id` and `client_secret`
- Callback URL: `https://your-domain.com/auth/bitbucket/callback`

## Multiple Provider Instances

For organizations using both public and self-hosted platforms:

```yaml
providers:
  github:
    api:
      base_url: https://api.github.com
    oauth:
      client_id: your-oauth-client-id
      client_secret: your-oauth-client-secret

  github_enterprise:
    api:
      base_url: https://ghe.company.com/api/v3
    oauth:
      client_id: your-ghe-oauth-client-id
      client_secret: your-ghe-oauth-client-secret
```

Each instance gets a unique hash. Specify which instance during OAuth:

```
https://your-domain.com/?provider=github&instance=<github-com-hash>
https://your-domain.com/?provider=github&instance=<ghe-hash>
```

## Security

- **CSRF Protection**: OAuth state parameter is cryptographically random (32 bytes)
- **Token Storage**: Access tokens stored in PostgreSQL (consider encryption for production)
- **Callback Validation**: State parameter validated on callback
- **Installation Records**: Uniquely indexed to prevent duplicates

## Troubleshooting

**"Invalid state parameter":**
- State used for CSRF protection
- Ensure cookies are enabled in the browser

**"Provider instance not found":**
- Instance hash doesn't match any configured provider
- Run `githook providers list` to get correct hash

**"Failed to exchange authorization code":**
- OAuth credentials incorrect
- Verify `client_id` and `client_secret` in config
- Check callback URL matches provider settings

**404 on callback:**
- Callback path must be `/auth/{provider}/callback`
- Verify `endpoint` is set correctly

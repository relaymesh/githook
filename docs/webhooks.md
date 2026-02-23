# Webhook Setup

Use provider-native webhook configuration to point to the Githooks endpoints. The
default paths are `/webhooks/github`, `/webhooks/gitlab`, and `/webhooks/bitbucket`.
You can override the path per provider instance with `webhook.path` in the provider
config YAML passed to `githook providers set`.

## GitHub (GitHub App)
1. Create a GitHub App in your org/user settings.
2. Set webhook URL to `https://<your-domain>/webhooks/github`.
3. Set the webhook secret in your provider instance config.
4. Create the provider instance via the CLI.
5. Subscribe to the events you need.
6. Deploy behind HTTPS.

Provider config example:
```yaml
webhook:
  secret: devsecret
app:
  app_id: 12345
  private_key_path: ./github.pem
oauth:
  client_id: your-client-id
  client_secret: your-client-secret
```

Create the provider:
```bash
githook --endpoint http://localhost:8080 providers set --provider github --config-file github.yaml
```

### GitHub Enterprise Server

GitHub Enterprise Server uses the same webhook endpoint (`/webhooks/github`).
Some versions send `X-Hub-Signature` (sha1) instead of `X-Hub-Signature-256`.
Githooks accepts either signature when the webhook secret is set.

Set `api.base_url` (and `web_base_url`) in the provider instance config to your GHE host:

```yaml
api:
  base_url: https://ghe.company.com/api/v3
  web_base_url: https://ghe.company.com
```

## GitLab
1. Go to **Settings → Webhooks** in your project/group.
2. Set URL to `https://<your-domain>/webhooks/gitlab`.
3. Set the webhook secret in the provider instance config (optional).
4. Select the events you want.
5. Save and test delivery.

## Bitbucket (Cloud)
1. Go to **Repository settings → Webhooks**.
2. Set URL to `https://<your-domain>/webhooks/bitbucket`.
3. Set the webhook secret in the provider instance config (optional, X-Hook-UUID).
4. Select the events you want.
5. Save and test delivery.

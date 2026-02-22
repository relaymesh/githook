# Webhook Setup

Use provider-native webhook configuration to point to the Githooks endpoints. If
you omit `providers.*.webhook.path` in config, defaults are `/webhooks/github`,
`/webhooks/gitlab`, and `/webhooks/bitbucket`.

## GitHub (GitHub App)
1. Create a GitHub App in your org/user settings.
2. Set webhook URL to `https://<your-domain>/webhooks/github`.
3. Set `GITHUB_WEBHOOK_SECRET` in your environment.
4. Set `providers.github.app.app_id` and `providers.github.app.private_key_path` in config.
4. Subscribe to the events you need.
5. Deploy behind HTTPS.

### GitHub Enterprise Server

GitHub Enterprise Server uses the same webhook endpoint (`/webhooks/github`).
Some versions send `X-Hub-Signature` (sha1) instead of `X-Hub-Signature-256`.
Githooks accepts either signature when the webhook secret is set.

Set `providers.github.api.base_url` (and `web_base_url`) to your GHE host.

## GitLab
1. Go to **Settings → Webhooks** in your project/group.
2. Set URL to `https://<your-domain>/webhooks/gitlab`.
3. Set `GITLAB_WEBHOOK_SECRET` (optional).
4. Select the events you want.
5. Save and test delivery.

## Bitbucket (Cloud)
1. Go to **Repository settings → Webhooks**.
2. Set URL to `https://<your-domain>/webhooks/bitbucket`.
3. Set `BITBUCKET_WEBHOOK_SECRET` (optional, X-Hook-UUID).
4. Select the events you want.
5. Save and test delivery.

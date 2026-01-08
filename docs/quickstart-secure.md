# Secure API Quickstart

This guide shows a secured setup using OAuth2/OIDC for the Connect RPC API, while keeping
webhooks and provider installs public.

## 1) Configure auth

```yaml
server:
  public_base_url: https://app.example.com

auth:
  oauth2:
    enabled: true
    issuer: https://<your-okta-domain>/oauth2/default
    audience: api://githooks
    required_scopes: ["githooks.rpc"]
    mode: auto
    client_id: ${OAUTH_CLIENT_ID}
    client_secret: ${OAUTH_CLIENT_SECRET}
```

## 2) Start the server

```bash
githooks serve --config config.yaml
```

## 3) Get a token

Machine token:
```bash
githooks auth token --config config.yaml
```

Human login:
```bash
githooks auth --config config.yaml
```
The login flow stores the token in the local cache automatically.
Open the URL, then store the token:
```bash
githooks auth store --config config.yaml --token <token> --expires-in 3600
```

## 4) Call RPCs

```bash
githooks --config config.yaml installations list --state-id <id>
```

## 5) Worker SDK

Workers that call the server API use the same `auth.oauth2` config. The SDK uses
client-credentials tokens automatically.

# githook Helm Chart

Deploys the githook server using:

- container image: `ghcr.io/relaymesh/githook`
- startup command: `githook serve --config <mounted-config>`
- service port: `8080`
- health endpoints: `/healthz` (liveness), `/readyz` (readiness)

## Install

```bash
helm upgrade --install githook ./charts/githook
```

## Configure server config file

By default, the chart creates a Secret from `values.yaml` under `config.data` and mounts it to `/app/config.yaml`.

Override with your own config file:

```bash
helm upgrade --install githook ./charts/githook \
  --set config.data="$(cat config.yaml)"
```

Use an existing Secret:

```bash
helm upgrade --install githook ./charts/githook \
  --set config.createSecret=false \
  --set config.existingSecret=my-githook-secret
```

## Key values

- `image.repository`, `image.tag`, `image.pullPolicy`
- `service.type`, `service.port`
- `ingress.enabled` and ingress host/path settings
- `autoscaling.enabled` for HPA
- `config.*` for server config source (Secret-only)

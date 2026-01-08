# Githooks Worker Helm Chart

## Install
```sh
helm install githooks-worker ./charts/githooks-worker \
  --set image.repository=ghcr.io/your-org/githooks \
  --set image.tag=latest \
  --set endpoint.baseURL=http://githooks:8080
```

## Configuration
- `configYaml` is stored in a Secret and mounted as `/app/config.yaml`.
- `endpoint.baseURL` is exported as `GITHOOKS_API_BASE_URL` for token resolution.
- `args` controls the worker command/flags.
- `extraConfigMapName` and `extraSecretName` can mount additional files.
- `extraEnv` allows injecting environment variables.

## Example
```sh
helm install githooks-worker ./charts/githooks-worker \
  --set image.repository=ghcr.io/your-org/githooks \
  --set image.tag=latest \
  --set-file configYaml=app.yaml \
  --set endpoint.baseURL=http://githooks:8080 \
  --set extraEnv[0].name=LOG_LEVEL \
  --set extraEnv[0].value=debug
```

# Githooks Worker Helm Chart

## Install
```sh
helm install githooks-worker ./charts/githooks-worker \
  --set image.repository=ghcr.io/your-org/githooks-worker \
  --set image.tag=latest \
  --set webhook.githubSecret=devsecret
```

## Configuration
- `configYaml` is stored in a Secret and mounted as `/app/config.yaml`.
- `extraConfigMapName` and `extraSecretName` can mount additional files.
- `extraEnv` allows injecting environment variables.

## Example
```sh
helm install githooks-worker ./charts/githooks-worker \
  --set image.repository=ghcr.io/your-org/githooks-worker \
  --set image.tag=latest \
  --set webhook.githubSecret=devsecret \
  --set-file configYaml=app.yaml \
  --set extraEnv[0].name=LOG_LEVEL \
  --set extraEnv[0].value=debug
```

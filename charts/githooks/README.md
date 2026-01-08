# Githooks Helm Chart

## Install
```sh
helm install githooks ./charts/githooks \
  --set image.repository=ghcr.io/your-org/githooks \
  --set image.tag=latest \
  --set webhook.githubSecret=devsecret
```

## Configuration
- `values.yaml` contains the full `configYaml` used by the server (stored as a Secret).
- Override it with `--set-file configYaml=app.yaml` or a values file.
- `webhook.githubSecret`, `webhook.gitlabSecret`, `webhook.bitbucketSecret` populate the matching environment variables.

## Example
```sh
helm install githooks ./charts/githooks \
  --set image.repository=ghcr.io/your-org/githooks \
  --set image.tag=latest \
  --set webhook.githubSecret=devsecret \
  --set-file configYaml=app.yaml
```

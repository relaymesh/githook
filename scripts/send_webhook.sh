#!/bin/sh
set -eu

usage() {
  echo "Usage: $0 <provider> <event> <payload.json>" >&2
  echo "Providers: github | gitlab | bitbucket" >&2
  exit 1
}

provider=${1:-}
event=${2:-}
payload_file=${3:-}

if [ -z "$provider" ] || [ -z "$event" ] || [ -z "$payload_file" ]; then
  usage
fi

body=$(cat "$payload_file")

case "$provider" in
  github)
    if [ -z "${GITHUB_WEBHOOK_SECRET:-}" ]; then
      echo "GITHUB_WEBHOOK_SECRET is required" >&2
      exit 1
    fi
    sig=$(printf '%s' "$body" | openssl dgst -sha1 -hmac "$GITHUB_WEBHOOK_SECRET" | sed 's/^.* //')
    curl -X POST http://localhost:8080/webhooks/github \
      -H "X-GitHub-Event: $event" \
      -H "X-Hub-Signature: sha1=$sig" \
      -H "Content-Type: application/json" \
      -d "$body"
    ;;
  gitlab)
    headers="-H X-Gitlab-Event: $event"
    if [ -n "${GITLAB_WEBHOOK_SECRET:-}" ]; then
      headers="$headers -H X-Gitlab-Token: $GITLAB_WEBHOOK_SECRET"
    fi
    curl -X POST http://localhost:8080/webhooks/gitlab \
      $headers \
      -H "Content-Type: application/json" \
      -d "$body"
    ;;
  bitbucket)
    headers="-H X-Event-Key: $event"
    if [ -n "${BITBUCKET_WEBHOOK_SECRET:-}" ]; then
      headers="$headers -H X-Hook-UUID: $BITBUCKET_WEBHOOK_SECRET"
    fi
    curl -X POST http://localhost:8080/webhooks/bitbucket \
      $headers \
      -H "Content-Type: application/json" \
      -d "$body"
    ;;
  *)
    usage
    ;;
esac

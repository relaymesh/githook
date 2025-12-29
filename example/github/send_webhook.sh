#!/bin/sh
set -eu

if [ -z "${GITHUB_WEBHOOK_SECRET:-}" ]; then
  echo "GITHUB_WEBHOOK_SECRET is required" >&2
  exit 1
fi

body_file="${1:-$(dirname "$0")/pull_request.json}"

body=$(cat "$body_file")

sig=$(printf '%s' "$body" | openssl dgst -sha1 -hmac "$GITHUB_WEBHOOK_SECRET" | sed 's/^.* //')

curl -X POST http://localhost:8080/webhooks/github \
  -H "X-GitHub-Event: pull_request" \
  -H "X-Hub-Signature: sha1=$sig" \
  -H "Content-Type: application/json" \
  -d "$body" -v

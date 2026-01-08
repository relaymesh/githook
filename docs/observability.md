# Observability

Githooks exposes lightweight observability signals that work with minimal setup.

## Request IDs

Incoming requests use or generate `X-Request-Id`. The server echoes it back in
responses and includes it in logs and published message metadata.

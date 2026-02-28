# Relaymesh Githook Python Worker SDK

The Python worker SDK mirrors the Go/TypeScript worker interfaces and connects to the Githook control plane for rules, drivers, event logs, and SCM credentials.

## Install

```bash
pip install relaymesh-githook
```

No additional packages are required.

## Quick Start

```python
import signal
import threading

from relaymesh_githook import New, NewRemoteSCMClientProvider, WithClientProvider, WithEndpoint

stop = threading.Event()

def shutdown(_signum, _frame):
    stop.set()

signal.signal(signal.SIGINT, shutdown)
signal.signal(signal.SIGTERM, shutdown)

wk = New(
    WithEndpoint("http://localhost:8080"),
    WithClientProvider(NewRemoteSCMClientProvider()),
)

def handle(ctx, evt):
    installation_id = evt.metadata.get("installation_id", "")
    print(f"topic={evt.topic} provider={evt.provider} type={evt.type} installation={installation_id}")
    if evt.payload:
        print(f"payload bytes={len(evt.payload)}")

wk.HandleRule("85101e9f-3bcf-4ed0-b561-750c270ef6c3", handle)

wk.Run(stop)
```

## Handler Signature

Handlers can accept either `(event)` or `(ctx, event)`:

```python
def handle(evt):
    print(evt.provider, evt.type)

def handle_with_ctx(ctx, evt):
    print(ctx.request_id, evt.topic)
```

## Using SCM Clients

```python
from relaymesh_githook import GitHubClient

def handler(ctx, evt):
    gh = GitHubClient(evt)
    if gh:
        user = gh.request_json("GET", "/user")
        print(user.get("login"))
```

## OAuth2 mode

Use `mode="client_credentials"` for worker OAuth2 token flow.

```python
from relaymesh_githook import OAuth2Config, WithOAuth2Config

wk = New(
    WithOAuth2Config(
        OAuth2Config(
            enabled=True,
            mode="client_credentials",
            client_id="your-client-id",
            client_secret="your-client-secret",
            token_url="https://issuer.example.com/oauth/token",
            scopes=["githook.read", "githook.write"],
        )
    )
)
```

## Environment Variables

The worker will read defaults from:

- `GITHOOK_ENDPOINT` or `GITHOOK_API_BASE_URL`
- `GITHOOK_API_KEY`
- `GITHOOK_TENANT_ID`

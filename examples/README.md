# Examples

Rule-id worker examples for Go, TypeScript, and Python.

- Go: `examples/go/worker/main.go`
- TypeScript: `examples/typescript/worker/index.ts`
- Python: `examples/python/worker/main.py`

Each example demonstrates:

- Provider-aware SCM handling for `github`, `gitlab`, and `bitbucket`
- Worker listener logging (`start`, `finish`, `error`) and automatic status updates (`success`/`failed`)
- Runtime knobs for concurrency and retries

Useful environment variables:

- `GITHOOK_ENDPOINT` (default `https://relaymesh.vercel.app/api/connect`)
- `GITHOOK_RULE_ID`
- `GITHOOK_CONCURRENCY` (default `4`)
- `GITHOOK_RETRY_COUNT` (default `1`)

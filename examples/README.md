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

- `RELAYMESH_ENDPOINT` (default `https://relaymesh.vercel.app/api/connect`)
- `RELAYMESH_RULE_ID`
- `RELAYMESH_CONCURRENCY` (default `4`)
- `RELAYMESH_RETRY_COUNT` (default `1`)

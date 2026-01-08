# Buf

This repo uses Buf to lint and generate protobuf code.

## Commands

```bash
buf lint
buf breaking --against ".git#branch=main"
buf generate
```

## Notes
- Protos live under `idl/`.
- Generated code goes to `gen/` when using `buf.gen.yaml`.

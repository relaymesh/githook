.PHONY: go ts py

GO_EXAMPLE=go/worker/main.go
TS_DIR=typescript/worker
PY_DIR=python/worker

GO_ENV?=
TS_ENV?=
PY_ENV?=

GO_RUN=go run ./examples/$(GO_EXAMPLE)
TS_RUN=cd examples/$(TS_DIR) && npm install && npm run start
PY_RUN=cd examples/$(PY_DIR) && python -m pip install -r requirements.txt && python main.py

go:
	$(GO_ENV) $(GO_RUN)

ts:
	$(TS_ENV) $(TS_RUN)

py:
	$(PY_ENV) $(PY_RUN)

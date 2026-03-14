.PHONY: go ts py

GO_EXAMPLE=go/worker/main.go
TS_DIR=typescript/worker
PY_DIR=python/worker

GO_ENV?=
TS_ENV?=
PY_ENV?=RELAYMESH_EXAMPLE_TIMEOUT=10

GO_RUN=go run ./$(GO_EXAMPLE)
TS_RUN=cd $(TS_DIR) && npm install && npm run start
PY_RUN=cd $(PY_DIR) && python -m pip install -r requirements.txt && python main.py

go:
	$(GO_ENV) $(GO_RUN)

ts:
	$(TS_ENV) $(TS_RUN)

py:
	$(PY_ENV) $(PY_RUN)

.PHONY: sdk-unit sdk-unit-go sdk-unit-ts sdk-unit-py sdk-build-ts sdk-test examples examples-go examples-ts examples-py bump-version

PYTHON ?= python3
NPM ?= npm

sdk-unit-go:
	go test ./sdk/go/worker

sdk-build-ts:
	$(NPM) --prefix sdk/typescript/worker run build

sdk-unit-ts:
	$(NPM) --prefix sdk/typescript/worker run test:unit

sdk-unit-py:
	PYTHONPATH=sdk/python/worker $(PYTHON) -m unittest discover -s sdk/python/worker/tests -p "test_*.py" -v

sdk-unit: sdk-unit-go sdk-unit-ts sdk-unit-py

sdk-test: sdk-unit

examples-go:
	$(MAKE) -C examples go

examples-ts:
	$(MAKE) -C examples ts

examples-py:
	$(MAKE) -C examples py

bump-version:
	@test -n "$(VERSION)" || (echo "VERSION is required. Usage: make bump-version VERSION=0.1.2" && exit 1)
	@$(PYTHON) scripts/bump_versions.py "$(VERSION)"

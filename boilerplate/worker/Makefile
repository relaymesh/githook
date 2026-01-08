SHELL := /bin/sh

ROOT ?= ../..
CONFIG ?= ./config.yaml
SCRIPTS ?= $(ROOT)/scripts
SERVER ?= $(ROOT)/main.go

.PHONY: run-worker run-server docker-build docker-run

run-worker:
	go run . -config $(CONFIG)

run-server:
	go run $(SERVER) serve --config $(CONFIG)

docker-build:
	docker build -f ./Dockerfile -t githooks-worker ../..

docker-run: docker-build
	docker run --rm \
		-e GITHUB_WEBHOOK_SECRET \
		-e GITHUB_APP_ID \
		-e GITHUB_PRIVATE_KEY_PATH \
		githooks-worker -config /app/config.yaml

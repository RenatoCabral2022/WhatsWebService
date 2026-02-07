.PHONY: all build test lint proto proto-lint clean docker-build docker-up docker-down

# ---- Meta ----
all: proto build test

# ---- Proto Generation ----
proto:
	buf generate

proto-lint:
	buf lint shared/protos

proto-breaking:
	buf breaking shared/protos --against '.git#subdir=shared/protos'

# ---- Go Services ----
build-control-plane:
	cd control-plane && go build -o ../bin/control-plane ./cmd/server

build-gateway:
	cd webrtc-gateway && go build -o ../bin/gateway ./cmd/gateway

build: build-control-plane build-gateway

# ---- Go Tests ----
test-control-plane:
	cd control-plane && go test ./...

test-gateway:
	cd webrtc-gateway && go test ./...

test-go: test-control-plane test-gateway

# ---- Python Tests ----
test-asr:
	cd inference/asr && python -m pytest tests/

test-tts:
	cd inference/tts && python -m pytest tests/

test-python: test-asr test-tts

# ---- All Tests ----
test: test-go test-python

# ---- Lint ----
lint-go:
	cd control-plane && go vet ./...
	cd webrtc-gateway && go vet ./...

lint-python:
	cd inference/asr && ruff check src/ tests/
	cd inference/tts && ruff check src/ tests/

lint: proto-lint lint-go lint-python

# ---- Docker ----
docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# ---- Clean ----
clean:
	rm -rf bin/
	rm -rf gen/

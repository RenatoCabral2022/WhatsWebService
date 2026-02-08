.PHONY: all build test lint proto proto-lint clean docker-build docker-up docker-down

# ---- Meta ----
all: proto build test

# ---- Proto Generation (Docker-based, no local tooling required) ----
proto: proto-go proto-python

proto-go:
	docker run --rm -v $(PWD):/workspace -w /workspace golang:1.22-alpine sh -c '\
		apk add --no-cache protobuf-dev && \
		go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.4 && \
		go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1 && \
		mkdir -p gen/go/whats/v1 && \
		protoc -I shared/protos \
			--go_out=gen/go --go_opt=paths=source_relative \
			--go-grpc_out=gen/go --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
			shared/protos/whats/v1/*.proto'

proto-python:
	docker run --rm -v $(PWD):/workspace -w /workspace python:3.11-slim sh -c '\
		pip install --quiet "grpcio-tools>=1.67.0,<1.72" "protobuf>=5.28.0,<6.0" && \
		mkdir -p gen/python/whats/v1 && \
		python -m grpc_tools.protoc -I shared/protos \
			--python_out=gen/python \
			--grpc_python_out=gen/python \
			shared/protos/whats/v1/*.proto && \
		touch gen/python/__init__.py gen/python/whats/__init__.py gen/python/whats/v1/__init__.py'

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

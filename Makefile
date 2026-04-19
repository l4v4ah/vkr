SHELL := /bin/bash
MODULE  := github.com/slava-kov/monitoring-system
PROTO_DIR := proto/telemetry
GEN_DIR   := gen/telemetry

.PHONY: all tidy proto build test integration-test lint migrate-up migrate-down \
        docker-up docker-down k8s-apply clean

all: tidy proto build

## Dependency management ──────────────────────────────────────────────────────

tidy:
	go mod tidy

## Protobuf code generation ───────────────────────────────────────────────────
## Requires: protoc, protoc-gen-go, protoc-gen-go-grpc
## Install:  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
##           go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

proto:
	mkdir -p $(GEN_DIR)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(GEN_DIR) --go_opt=paths=source_relative \
		--go-grpc_out=$(GEN_DIR) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/telemetry.proto

## Build ───────────────────────────────────────────────────────────────────────

build:
	CGO_ENABLED=0 go build ./...

build-collector:
	CGO_ENABLED=0 go build -o bin/collector ./cmd/collector

build-aggregator:
	CGO_ENABLED=0 go build -o bin/aggregator ./cmd/aggregator

build-api:
	CGO_ENABLED=0 go build -o bin/api ./cmd/api

## Testing ─────────────────────────────────────────────────────────────────────

test:
	go test -count=1 -race -timeout=60s ./cmd/...

integration-test:
	go test -count=1 -race -timeout=120s -tags=integration ./internal/storage/...

## Linting ─────────────────────────────────────────────────────────────────────

lint:
	golangci-lint run --timeout=5m ./...

## Database migrations ─────────────────────────────────────────────────────────
## Requires: migrate CLI  https://github.com/golang-migrate/migrate

DB_URL ?= postgres://monitoring:monitoring@localhost:5432/monitoring?sslmode=disable

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

## Docker Compose ──────────────────────────────────────────────────────────────

docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker-compose.yml down -v

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f

## Kubernetes ──────────────────────────────────────────────────────────────────

k8s-apply:
	kubectl apply -f deployments/k8s/

k8s-delete:
	kubectl delete -f deployments/k8s/

## Misc ────────────────────────────────────────────────────────────────────────

clean:
	rm -rf bin/ gen/

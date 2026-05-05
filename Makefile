.PHONY: run build test test-verbose lint docker-build docker-run clean demo-fallback demo-timeout

BINARY   := car-payment-gateway
IMAGE    := car-payment-gateway:latest
PORT     ?= 3000

# Sobrescreve PORT (e outras chaves) a partir de .env, se existir
-include .env
export PORT
export ADDR
export REFERENCE_DATE

## Run the server locally (hot-reload friendly: just re-run make run)
run:
	PORT=$(PORT) REFERENCE_DATE=$(REFERENCE_DATE) go run ./cmd/api/...

## Build the production binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/$(BINARY) ./cmd/api/...

## Run all tests
test:
	go test ./... -count=1

## Run all tests with verbose output
test-verbose:
	go test ./... -v -count=1

## Run all tests with race detector
test-race:
	go test ./... -race -count=1

## Show test coverage
coverage:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out

## Vet and staticcheck (no third-party linter required)
lint:
	go vet ./...

## Build the Docker image
docker-build:
	docker build -t $(IMAGE) .

## Run the service in Docker
docker-run: docker-build
	docker run --rm -p $(PORT):$(PORT) -e PORT=$(PORT) -e ADDR=$(ADDR) -e REFERENCE_DATE=$(REFERENCE_DATE) $(IMAGE)

## Demo fallback: start with MockFailing provider first in chain (port 3002)
demo-fallback:
	PORT=3002 ENABLE_MOCK_FAILING=true REFERENCE_DATE=$(REFERENCE_DATE) go run ./cmd/api/...

## Demo timeout: first provider blocks until per-attempt deadline (then fallback, port 3003)
demo-timeout:
	PORT=3003 ENABLE_MOCK_SLOW=true REFERENCE_DATE=$(REFERENCE_DATE) go run ./cmd/api/...

## Clean build artefacts
clean:
	rm -rf bin/ coverage.out

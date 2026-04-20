.PHONY: run build test test-verbose lint docker-build docker-run clean

BINARY   := car-payment-gateway
IMAGE    := car-payment-gateway:latest
PORT     ?= 8080

## Run the server locally (hot-reload friendly: just re-run make run)
run:
	go run ./cmd/api/...

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
	docker run --rm -p $(PORT):$(PORT) -e PORT=$(PORT) $(IMAGE)

## Demo fallback: start with MockFailing provider first in chain
demo-fallback:
	ENABLE_MOCK_FAILING=true go run ./cmd/api/...

## Clean build artefacts
clean:
	rm -rf bin/ coverage.out

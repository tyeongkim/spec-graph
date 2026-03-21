BINARY   := spec-graph
MODULE   := github.com/taeyeong/spec-graph
BUILD_DIR := bin

GO       := go
GOFLAGS  :=
LDFLAGS  :=

.PHONY: all build test lint fmt vet tidy clean check run

all: check build

## Build
build:
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/spec-graph

run: build
	./$(BUILD_DIR)/$(BINARY)

## Quality
test:
	$(GO) test $(GOFLAGS) ./...

test-v:
	$(GO) test $(GOFLAGS) -v ./...

test-cover:
	$(GO) test $(GOFLAGS) -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

lint: vet
	@which golangci-lint > /dev/null 2>&1 || { echo "golangci-lint not found. Install: https://golangci-lint.run/welcome/install/"; exit 1; }
	golangci-lint run ./...

tidy:
	$(GO) mod tidy

## Combo
check: fmt vet test

## Clean
clean:
	rm -rf $(BUILD_DIR) coverage.out

BINARY_NAME=seraph
GO=go
GOLANGCI_LINT?=golangci-lint
CATALOG_DIR?=catalog/packs
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X github.com/fabianoflorentino/seraph/internal/version.Version=$(VERSION) -X github.com/fabianoflorentino/seraph/internal/version.Commit=$(COMMIT) -X github.com/fabianoflorentino/seraph/internal/version.Date=$(DATE)"

.PHONY: all build clean test coverage lint lint-fix fmt vet run sync validate preview check help

.DEFAULT_GOAL := help

## all: Run clean, lint, test and build
all: clean lint test build

## build: Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/seraph

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html

## test: Run all tests
test:
	@echo "Running tests..."
	$(GO) test -race -shuffle=on -count=1 ./...

## coverage: Run tests and report total coverage
coverage:
	$(GO) test -race -shuffle=on -count=1 -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out | tail -1

## lint: Run linter
lint:
	@echo "Running linter..."
	$(GOLANGCI_LINT) run ./...

## lint-fix: Run linter and auto-fix issues
lint-fix:
	@echo "Running linter with auto-fix..."
	$(GOLANGCI_LINT) run --fix ./...

## fmt: Format code and run vet
fmt:
	$(GO) fmt ./...
	$(GO) vet ./...

## vet: Run go vet
vet:
	$(GO) vet ./...

## run: Build and run the application
run: build
	./bin/$(BINARY_NAME)

## sync: Collect services and write the dashboard snapshot
sync: build
	./bin/$(BINARY_NAME) sync --catalog $(CATALOG_DIR)

## validate: Validate the catalog only
validate: build
	./bin/$(BINARY_NAME) validate --catalog $(CATALOG_DIR)

## preview: Serve the static site locally on http://localhost:8000
preview:
	python3 -m http.server -d site 8000

## check: Run the full local gate (fmt, vet, lint, test, coverage)
check: fmt lint test coverage

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@awk -F ': ' '/^## / {sub(/^## /, ""); printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

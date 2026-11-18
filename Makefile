BINARY_NAME=seraph
GO=go
GOLANGCI_LINT?=golangci-lint
CATALOG_DIR?=catalog/packs
DOCKER_COMPOSE?=docker compose
DEV_INTERVAL?=60
SITE_PORT?=1314
DEV_ENV=env UID=$(shell id -u) GID=$(shell id -g) SITE_PORT=$(SITE_PORT) SERAPH_DEV_INTERVAL=$(DEV_INTERVAL)
HUGO_IMAGE=hugomods/hugo:0.158.0
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X github.com/fabianoflorentino/seraph/internal/version.Version=$(VERSION) -X github.com/fabianoflorentino/seraph/internal/version.Commit=$(COMMIT) -X github.com/fabianoflorentino/seraph/internal/version.Date=$(DATE)"

.PHONY: all build clean test coverage lint lint-fix fmt vet run sync validate site-build preview check \
	dev/up dev/down dev/logs dev/ps dev/shell dev/collect dev/rebuild dev/reset help

.DEFAULT_GOAL := help

## all: Run clean, lint, test and build
all: clean lint test build

## build: Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/seraph

## clean: Clean build artifacts and generated site output
clean:
	@echo "Cleaning..."
	rm -rf bin/ site/public/ site/resources/ site/.hugo_build.lock
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

## check: Run the full local gate (fmt, vet, lint, test, coverage)
check: fmt lint test coverage

# ---- Local development stack (Hugo + collector) ---------------------------

## dev/up: Build and start Hugo and the collector on http://localhost:1314
dev/up:
	$(DEV_ENV) $(DOCKER_COMPOSE) up -d --build
	@echo "site: http://localhost:$(SITE_PORT)  (logs: make dev/logs)"

## dev/down: Stop the local development stack
dev/down:
	$(DEV_ENV) $(DOCKER_COMPOSE) down

## dev/logs: Follow the local development stack logs
dev/logs:
	$(DEV_ENV) $(DOCKER_COMPOSE) logs -f

## dev/ps: Show the local development stack status
dev/ps:
	$(DEV_ENV) $(DOCKER_COMPOSE) ps

## dev/shell: Open a shell in the collector container
dev/shell:
	$(DEV_ENV) $(DOCKER_COMPOSE) exec collector sh

## dev/collect: Run one collection cycle immediately
dev/collect:
	$(DEV_ENV) $(DOCKER_COMPOSE) exec collector ./bin/seraph sync --catalog catalog/packs

## dev/rebuild: Rebuild and restart the collector after Go changes
dev/rebuild:
	$(DEV_ENV) $(DOCKER_COMPOSE) build --no-cache collector
	$(DEV_ENV) $(DOCKER_COMPOSE) up -d --force-recreate collector

## dev/reset: Stop the stack and remove the generated snapshot
dev/reset:
	$(DEV_ENV) $(DOCKER_COMPOSE) down --remove-orphans
	rm -f site/static/data/state.json

# ---- Static site ----------------------------------------------------------

## site-build: Build the production site into site/public
site-build:
	$(DEV_ENV) $(DOCKER_COMPOSE) run --rm --no-deps site hugo --minify --gc --cleanDestinationDir

## preview: Build the site and serve site/public on http://localhost:8000
preview: site-build
	python3 -m http.server -d site/public 8000

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@awk -F ': ' '/^## / {sub(/^## /, ""); printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

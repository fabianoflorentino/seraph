BINARY_NAME=seraph
GO=go
GOLANGCI_LINT?=golangci-lint
CATALOG_DIR?=catalog/packs
CATALOG_PACK?=catalog/packs/global.yaml
CATALOG_SLUG?=
CATALOG_NAME?=
CATALOG_CATEGORY?=
CATALOG_OFFICIAL_URL?=
CATALOG_STATUS_URL?=
CATALOG_SOURCE_TYPE?=
CATALOG_SOURCE_URL?=
CATALOG_COMPONENT?=
CATALOG_FORMAT?=
CATALOG_REGION?=global
CATALOG_TARGET?=
CATALOG_INSPECT_ARGS?=
CATALOG_AUDIT_ARGS?=
CATALOG_OPTIONS=$(if $(CATALOG_OFFICIAL_URL),--official-url "$(CATALOG_OFFICIAL_URL)",) $(if $(CATALOG_STATUS_URL),--status-url "$(CATALOG_STATUS_URL)",) $(if $(CATALOG_COMPONENT),--component "$(CATALOG_COMPONENT)",) $(if $(CATALOG_FORMAT),--format "$(CATALOG_FORMAT)",)
DOCKER_COMPOSE?=docker compose
DEV_INTERVAL?=60
SITE_PORT?=1314
PREVIEW_PORT?=8000
PREVIEW_DIR?=.preview
PREVIEW_LOG?=$(PREVIEW_DIR)/server.log
PREVIEW_PID?=$(PREVIEW_DIR)/server.pid
DEV_ENV=env UID=$(shell id -u) GID=$(shell id -g) SITE_PORT=$(SITE_PORT) SERAPH_DEV_INTERVAL=$(DEV_INTERVAL)
HUGO_IMAGE=hugomods/hugo:0.158.0
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT?=$(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-X github.com/fabianoflorentino/seraph/internal/version.Version=$(VERSION) -X github.com/fabianoflorentino/seraph/internal/version.Commit=$(COMMIT) -X github.com/fabianoflorentino/seraph/internal/version.Date=$(DATE)"

.PHONY: all build clean test coverage lint lint-fix fmt vet run sync validate catalog-add \
	catalog-inspect catalog-audit site-build \
	preview preview/logs preview/status preview/stop check \
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

## catalog-add: Add a service to the catalog (CATALOG_SLUG=... CATALOG_NAME=...)
catalog-add: build
	@if [ -z "$(CATALOG_SLUG)" ] || [ -z "$(CATALOG_NAME)" ] || [ -z "$(CATALOG_CATEGORY)" ] || [ -z "$(CATALOG_SOURCE_TYPE)" ] || [ -z "$(CATALOG_SOURCE_URL)" ]; then \
		echo "usage: make catalog-add CATALOG_SLUG=... CATALOG_NAME=... CATALOG_CATEGORY=... CATALOG_SOURCE_TYPE=... CATALOG_SOURCE_URL=... [CATALOG_PACK=...] [CATALOG_OFFICIAL_URL=...] [CATALOG_STATUS_URL=...] [CATALOG_COMPONENT=...] [CATALOG_FORMAT=...] [CATALOG_REGION=...]" >&2; \
		exit 1; \
	fi
	./bin/$(BINARY_NAME) catalog add \
		--pack "$(CATALOG_PACK)" \
		--slug "$(CATALOG_SLUG)" \
		--name "$(CATALOG_NAME)" \
		--category "$(CATALOG_CATEGORY)" \
		--source-type "$(CATALOG_SOURCE_TYPE)" \
		--source-url "$(CATALOG_SOURCE_URL)" \
		--region "$(CATALOG_REGION)" \
		$(CATALOG_OPTIONS)

## catalog-inspect: Discover status sources (CATALOG_TARGET=example.com)
catalog-inspect: build
	@if [ -z "$(CATALOG_TARGET)" ]; then \
		echo "usage: make catalog-inspect CATALOG_TARGET=example.com [CATALOG_INSPECT_ARGS='--json']" >&2; \
		exit 1; \
	fi
	./bin/$(BINARY_NAME) catalog inspect "$(CATALOG_TARGET)" $(CATALOG_INSPECT_ARGS)

## catalog-audit: Collect and report every catalog source
catalog-audit: build
	./bin/$(BINARY_NAME) catalog audit $(CATALOG_AUDIT_ARGS)

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

## preview: Build the site and serve it in the background on http://localhost:8000
preview: site-build
	@mkdir -p "$(PREVIEW_DIR)"
	@if [ -f "$(PREVIEW_PID)" ] && kill -0 "$$(cat "$(PREVIEW_PID)")" 2>/dev/null; then \
		echo "preview already running at http://localhost:$(PREVIEW_PORT) (logs: make preview/logs)"; \
	else \
		rm -f "$(PREVIEW_PID)"; \
		nohup python3 -m http.server -d site/public $(PREVIEW_PORT) >"$(PREVIEW_LOG)" 2>&1 & \
		echo $$! > "$(PREVIEW_PID)"; \
		sleep 1; \
		if kill -0 "$$(cat "$(PREVIEW_PID)")" 2>/dev/null; then \
			echo "preview: http://localhost:$(PREVIEW_PORT)  (logs: make preview/logs, stop: make preview/stop)"; \
		else \
			echo "preview failed to start; last log lines:" >&2; \
			tail -n 20 "$(PREVIEW_LOG)" >&2; \
			rm -f "$(PREVIEW_PID)"; \
			exit 1; \
		fi; \
	fi

## preview/logs: Follow the preview server log
preview/logs:
	@mkdir -p "$(PREVIEW_DIR)"
	@touch "$(PREVIEW_LOG)"
	tail -f "$(PREVIEW_LOG)"

## preview/status: Show whether the preview server is running
preview/status:
	@if [ -f "$(PREVIEW_PID)" ] && kill -0 "$$(cat "$(PREVIEW_PID)")" 2>/dev/null; then \
		echo "preview running at http://localhost:$(PREVIEW_PORT) (pid $$(cat "$(PREVIEW_PID)"))"; \
	else \
		echo "preview stopped"; \
	fi

## preview/stop: Stop the background preview server
preview/stop:
	@if [ -f "$(PREVIEW_PID)" ]; then \
		pid=$$(cat "$(PREVIEW_PID)"); \
		if kill -0 "$$pid" 2>/dev/null; then \
			kill "$$pid"; \
			echo "stopped preview (pid $$pid)"; \
		else \
			echo "preview not running"; \
		fi; \
		rm -f "$(PREVIEW_PID)"; \
	else \
		echo "preview not running"; \
	fi

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@awk -F ': ' '/^## / {sub(/^## /, ""); printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

# PostgreSQL TUI Manager Makefile
SHELL := /bin/bash
APP_NAME := postgres-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
GO ?= go

.PHONY: all build install clean test test-cover test-cover-check lint fmt run start demo-run release snapshot demo \
	docker-up docker-down docker-seed

all: build

build:
	$(GO) build $(LDFLAGS) -o bin/$(APP_NAME) ./

install:
	$(GO) install $(LDFLAGS) ./

clean:
	rm -rf bin/
	rm -rf dist/

test:
	$(GO) test -v -race ./...

test-cover:
	$(GO) test -v -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

MIN_COVERAGE ?= 60

test-cover-check:
	@set -euo pipefail; \
		pkgs=$$($(GO) list ./... | grep -v '/examples/'); \
		$(GO) test -v -race -coverprofile=coverage.out $$pkgs; \
		total=$$($(GO) tool cover -func=coverage.out | awk '/total:/ {print $$NF}' | tr -d '%'); \
		echo "Total coverage: $${total}% (minimum $(MIN_COVERAGE)%)"; \
		if (( $$(echo "$$total < $(MIN_COVERAGE)" | bc -l) )); then \
			echo "FAIL: coverage $${total}% is below $(MIN_COVERAGE)%"; \
			exit 1; \
		fi; \
		for dir in internal/cmd internal/types internal/service; do \
			$(GO) test -coverprofile=/tmp/postgres-tui-pkg.out ./$$dir >/dev/null; \
			pt=$$($(GO) tool cover -func=/tmp/postgres-tui-pkg.out | awk '/total:/ {print $$NF}' | tr -d '%'); \
			echo "$$dir coverage: $${pt}%"; \
		done; \
		echo "Coverage OK"

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

run: build
	./bin/$(APP_NAME)

# Full local demo: docker postgres + seed data + TUI (uses saved Local Demo connection).
demo-run: docker-up docker-seed build
	./bin/$(APP_NAME)

start: run

build-all:
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-amd64 ./
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-arm64 ./
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-linux-arm64 ./
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe ./

release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean

dev-deps:
	go install github.com/goreleaser/goreleaser/v2@v2.13.1

docker-up:
	docker compose -f examples/postgres/docker-compose.yml up -d

docker-down:
	docker compose -f examples/postgres/docker-compose.yml down

docker-seed:
	$(GO) run ./examples/seed -host localhost -port 5432 -user postgres -password postgres -database demo

## Render README demo GIF (VHS). Unset NO_COLOR so lipgloss emits colors.
demo: docker-up docker-seed
	$(GO) build -o bin/$(APP_NAME) ./
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/demo.tape
	@echo "Demo GIF written to docs/main.gif"

## Static UI screenshots via VHS (no live postgres required for connections screen)
demo-ui:
	$(GO) build -o bin/$(APP_NAME) ./
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/ui-connections.tape
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/ui-browser.tape
	@echo "UI tapes complete — check docs/*.png"

help:
	@echo "Available targets:"
	@echo "  build, test, lint, fmt, run, demo-run, docker-up, docker-seed, demo, demo-ui"

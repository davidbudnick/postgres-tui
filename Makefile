# PostgreSQL TUI Manager Makefile
SHELL := /bin/bash
APP_NAME := postgres-tui
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"
GO ?= go

.PHONY: all build install clean test test-cover test-cover-check lint fmt run start demo-run release snapshot demo \
	demo-ui docker-up docker-down docker-seed help dev-deps build-all

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

## Run tests with coverage (examples excluded from coverprofile)
test-cover:
	pkgs=$$($(GO) list ./... | grep -v '/examples/'); \
	$(GO) test -v -race -coverprofile=coverage.out $$pkgs
	$(GO) tool cover -html=coverage.out -o coverage.html

## Fail if any function is below 100% coverage (redis-tui style)
test-cover-check:
	@pkgs=$$($(GO) list ./... | grep -v '/examples/'); \
		$(GO) test -v -race -coverprofile=coverage.out $$pkgs && \
		set -euo pipefail; \
		FAILED=0; \
		while IFS= read -r line; do \
			func=$$(echo "$$line" | awk '{print $$2}'); \
			pct=$$(echo "$$line" | awk '{print $$NF}' | tr -d '%'); \
			if [[ "$$func" == "(statements)" ]]; then \
				continue; \
			fi; \
			if (( $$(echo "$$pct < 100.0" | bc -l) )); then \
				location=$$(echo "$$line" | awk '{print $$1}'); \
				echo "FAIL: Function $$func at $$location coverage is $${pct}%, required 100%"; \
				FAILED=1; \
			fi; \
		done < <(go tool cover -func=coverage.out); \
		if [[ $$FAILED -eq 1 ]]; then \
			exit 1; \
		fi; \
		echo "All functions at 100% coverage"

lint:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

run: build
	./bin/$(APP_NAME)

# Full local demo: docker postgres + seed data + TUI
demo-run: docker-up docker-seed build
	./bin/$(APP_NAME)

start: run

build-all:
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-amd64 ./
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-darwin-arm64 ./
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-linux-amd64 ./
	GOOS=linux GOARCH=arm64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-linux-arm64 ./
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o bin/$(APP_NAME)-windows-amd64.exe ./

# Local goreleaser only — production releases are tag-driven via GitHub Actions
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

## Render README demo GIF (VHS)
demo: docker-up docker-seed
	$(GO) build -o bin/$(APP_NAME) ./
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/demo.tape
	@echo "Demo GIF written to docs/main.gif"

## Static UI screenshots via VHS
demo-ui:
	$(GO) build -o bin/$(APP_NAME) ./
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/ui-connections.tape
	env -u NO_COLOR COLORTERM=truecolor TERM=xterm-256color CLICOLOR_FORCE=1 vhs docs/ui-browser.tape
	@echo "UI tapes complete — check docs/*.png"

help:
	@echo "Available targets:"
	@echo "  build, test, test-cover-check, lint, fmt, run, demo-run, docker-up, docker-seed, demo, demo-ui, release"

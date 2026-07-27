# CLAUDE.md

## Project Overview

PostgreSQL TUI Manager — a terminal user interface for managing PostgreSQL databases, built with Go and Bubble Tea. Sibling to redis-tui and es-tui.

## Build & Test Commands

```bash
make build              # Build binary to bin/postgres-tui
make test               # Run tests: go test -v -race ./...
make test-cover         # Tests with coverage report
make lint               # Run go vet
make fmt                # Format code
make run                # Build and run
make docker-up          # Start demo Postgres (:5432)
make docker-seed        # Seed demo schema/data
make docker-down        # Stop demo Postgres
make demo               # Render README GIF via vhs
make demo-ui            # Capture UI screenshots via vhs
```

## Architecture

```
main.go                    # Entry point, CLI flag parsing, config init
internal/
  cmd/                     # Bubble Tea command factories (return tea.Cmd)
  ui/                      # Bubble Tea UI (Model/Update/View pattern)
  pg/                      # PostgreSQL client (pgx)
  types/                   # Shared type definitions and messages
  db/                      # Config persistence (~/.config/postgres-tui/config.json)
  service/                 # Interfaces (ConfigService, PGService) and DI container
  testutil/                # Test helpers and mock implementations
```

**Bubble Tea message flow**: KeyMsg → handleKeyPress() → Command (tea.Cmd) → async execution → Message (tea.Msg) → Update() → View()

## Code Conventions

- **Go version**: 1.26.5 (set in go.mod)
- **Package names**: lowercase, single word
- **Receivers**: short names (`c *Client`, `m Model`, `cfg *Config`)
- **Message types**: PascalCase with `Msg` suffix
- **Command methods**: return `tea.Cmd`
- **Error handling**: wrap with `fmt.Errorf("context: %w", err)`
- **Logging**: `log/slog` structured logging
- **Dependency injection**: services via `Commands{config, pg}`

## Guardrails

- All PostgreSQL operations go through `internal/pg/` — no raw pgx in UI/cmd
- Passwords persisted in config (mode 0600)
- New commands go through `Commands` with injected services
- Message types use `Msg` suffix in `types/messages.go`

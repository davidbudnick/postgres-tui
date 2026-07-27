# PostgreSQL TUI Manager

[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A fast, keyboard-first terminal UI for PostgreSQL — built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea). Sibling to [redis-tui](https://github.com/davidbudnick/redis-tui) and [es-tui](https://github.com/davidbudnick/es-tui).

![Main Screenshot](docs/main.gif)

## Features

- **Instance manager** — save and switch between multiple PostgreSQL servers
- **Database picker** — choose a database after connecting
- **Sidebar navigation** — schemas, tables, views, sequences, functions, types, extensions
- **Table browser** — paginated rows, describe (columns / indexes / constraints)
- **SQL query editor** — run queries, view results, export CSV
- **Activity** — `pg_stat_activity` style live backends
- **Server info** — version, encoding, connections, uptime
- **Read-only mode** — block mutating statements
- **SSL modes** — disable through verify-full
- **Fully keyboard-driven** — vim-style navigation, command palette

## Screenshots

SQL editor — syntax colors, smart complete, live results:

![SQL query](docs/query.png)

Table data grid (blue selection + typed cells):

![Table data](docs/table-data.png)

ER diagram for the current schema:

![ERD](docs/erd.png)

## Quick Install

```bash
# From source (Go 1.26+)
git clone https://github.com/davidbudnick/postgres-tui.git
cd postgres-tui
make build
./bin/postgres-tui
```

## Terminal font

**No special font package required.** TOOLS tags and chrome use plain ASCII (`> * + @ #`, etc.), so the default monospace font in iTerm2, Terminal.app, Kitty, WezTerm, Ghostty, or Alacritty is fine. A Nerd Font is optional eye candy only—not a dependency.

## Usage

```bash
# Interactive instance manager
postgres-tui

# Quick connect
postgres-tui --host localhost --user postgres --password secret --database mydb

# Read-only
postgres-tui --host db.example.com --read-only --sslmode require
```

### Keybindings

| Key | Action |
|-----|--------|
| `a` | Add instance |
| `enter` | Connect / open |
| `tab` | Toggle sidebar / main |
| `j` / `k` | Navigate |
| `D` | Describe object |
| `;` | Open SQL query editor (primary) |
| `:` | Open SQL query editor (alias) |
| `ctrl+enter` / `ctrl+e` | Run SQL in the editor |
| `A` | Activity |
| `i` | Server info |
| `/` | Filter objects |
| `?` | Help |
| `esc` | Back |
| `q` | Quit |

### SQL queries

1. Connect and open a database (sidebar tree loads).
2. **Open the editor** with **`;`** (or `:` / TOOLS → Query / palette).
3. Type SQL — keywords, strings, numbers, and functions are **syntax-colored as you type**; autocomplete suggests tables, columns, and keywords (`tab` to accept, `↑`/`↓` to cycle).
4. **Run** with **`ctrl+enter`** or **`ctrl+e`** (also `f5` / `ctrl+r` in many terminals).
5. Results appear below; `tab` moves focus editor ↔ results when no suggestion is open.
6. `x` exports the result grid as CSV; `esc` returns to the tree.

Prefill: with a table/view selected, `;` inserts `SELECT * FROM schema.table LIMIT 100;` and runs it once.

## Local Demo

```bash
make demo-run
```

That starts Postgres, seeds sample data, and launches the TUI. On first run the app seeds two saved instances:

| Name | Database | Notes |
|------|----------|--------|
| **Local Demo** | `demo` | Full seed data (tables, orders, analytics) |
| **Analytics (RO)** | `postgres` | Read-only browse |

Then: select **Local Demo** → `enter` → pick `demo` → browse tables → `enter` for data / `D` structure / `;` query (`ctrl+enter` to run).

```bash
# Or step by step:
make docker-up && make docker-seed && make run

# Or CLI quick-connect (skips the instance list):
./bin/postgres-tui --host localhost --user postgres --password postgres --database demo --sslmode disable
```

## Recording demos

The hero GIF (`docs/main.gif`) and screenshot PNGs under `docs/` are produced with [VHS](https://github.com/charmbracelet/vhs):

```bash
# Full animated README GIF (Docker Postgres + vhs)
make demo

# Refresh static PNGs used in the gallery above
make demo-ui
```

`docs/demo.tape` drives the hero GIF; `make demo-ui` refreshes the stills above.

## License

MIT

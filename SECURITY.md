# Security Policy

## Credential handling

- Passwords entered in the connection form are stored in `~/.config/postgres-tui/config.json` with mode `0600` (owner read/write only).
- Prefer environment variables or interactive entry over CLI flags for secrets.
- CLI flags (`--password` / `-a`) print a warning: secrets appear in the process list.

## Reporting

Please open a private security advisory or email the maintainer if you discover a vulnerability.

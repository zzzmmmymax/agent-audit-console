# Stage 0 completion report

## Delivered

- Initialized the Go module and Git repository.
- Created the required directory structure and placeholder executable entry
  points without entering Stage 1 functionality.
- Defined all required event fields and six required event kinds.
- Added event validation plus repository/query contracts.
- Added deterministic SHA-256 sealing and verification for per-run hash chains.
- Designed SQLite `runs`, `events`, `snapshots`, and `rollback_records` tables,
  constraints, indexes, JSON validation, WAL, foreign keys, and full sync.
- Added contracts for capture, content-addressed snapshots, policy evaluation,
  rollback planning/execution, and the local HTTP service.
- Added tests covering valid chains, tampering, broken links, stable hashing,
  invalid predecessor hashes, event validation, and schema essentials.
- Documented architecture, event schema, security boundaries, local storage, and
  recovery requirements.

## Deliberately deferred

Cobra commands, command execution, file watching, diffs, snapshot I/O, restore,
SQLite repository implementation/driver, chi HTTP handlers, React/Vite UI, MCP
tools, YAML policies, approvals, redaction, and audit exports belong to later
stages and are not included.

## Verification

Run:

```shell
go test ./...
```

Completed successfully with the official Go 1.23 container:

- `gofmt -w ./cmd ./internal`
- `go vet ./...`
- `go test ./...`

The DDL was also executed against in-memory SQLite 3.45.3; all four core tables
were created successfully.

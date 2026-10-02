# Stage 1 completion report

## Delivered

- `audit run -- COMMAND`: records command arguments, absolute working directory,
  start/end time, duration, exit code, stdout, and stderr.
- Workspace capture: compares regular files before and after controlled command
  execution and emits added, modified, and deleted `file_change` events with
  SHA-256 hashes and unified diffs. `.git`, `.agent-audit`, and `node_modules`
  are excluded from file scanning.
- Snapshot store: streams data into a SHA-256 content-addressed object store,
  deduplicates identical bytes, verifies content before restore, and uses a
  same-directory replacement strategy.
- Recovery: `audit restore FILE` restores modified/deleted content or removes a
  file that did not exist before the audited command. A safety snapshot,
  rollback record, and rollback event are created.
- CLI: `audit run`, `audit log`, `audit show RUN_ID`, and `audit restore FILE`.
- Local Web: chi API plus an embedded React/TypeScript/Vite application showing
  task lists, event timelines, commands, outputs, test exit status, file changes,
  and split diffs using `react-diff-view`.
- MCP stdio server using the official Go SDK with `start_run`, `record_action`,
  `get_run_summary`, and `request_rollback`.
- Pure-Go SQLite persistence with WAL and atomic sequence/hash-head updates.

## Verification

- `npm run build`: passed.
- `go test ./...`: passed, including real subprocess capture and three restore
  paths (modified, deleted, and newly added files).
- MCP in-memory transport test: passed and exposes exactly four tools.
- CLI smoke test: passed for `run`, `log`, and `show`; stored chain verified.
- HTTP smoke test: `/healthz`, `/api/runs`, and embedded `/` returned success.
- `go vet ./...` and final build are part of the delivery verification.

The race detector was attempted but is unavailable in this Windows installation
because it requires CGO and a C compiler. The project itself uses a pure-Go
SQLite driver and does not otherwise require CGO.

## Known MVP boundaries

- File capture compares pre/post command state; transient changes that are both
  created and removed during one command are not observable.
- Only activity routed through the CLI or MCP boundary is audited.
- Output redaction, YAML risk rules, approval gates, and export are Stage 2.
- MCP rollback requests are recorded as pending; approval/execution policy is a
  Stage 2 concern. CLI restore remains an explicit local user action.

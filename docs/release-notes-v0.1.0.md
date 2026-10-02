# Agent Audit Console v0.1.0

Initial stable release of the local-first Agent audit recorder and control
console.

## Highlights

- Local-first audit data in SQLite with WAL mode.
- Agent-independent event model for command, file-change, Git, MCP, approval,
  and rollback activity.
- `audit` CLI command capture, run inspection, integrity verification, export,
  diagnostics, and conflict-aware restore.
- File diffs and SHA-256 content-addressed snapshots with integrity checks,
  deduplication, and safe restoration.
- Tamper-evident event hash chains with full-run verification.
- YAML policy engine, risk levels, approval states, and layered team policy.
- MCP stdio tools for starting runs, recording actions, reading summaries, and
  requesting rollback.
- Embedded local Web console with paginated timelines and diff presentation.
- Agent adapters for Codex, Claude Code, Cursor, and custom integrations.
- Viewer/operator/admin bearer-token RBAC for the HTTP API.
- Explicit operator-invoked remote synchronization and JSON/HTML audit export.

## Security

- Audit state is local by default and `auditd` binds to loopback by default.
- Event evidence, exports, Web responses, and sync bundles apply secret
  redaction; exact recovery snapshots can still contain secrets.
- Event-chain verification detects modification, insertion, deletion, sequence
  changes, and broken links against the trusted run head.
- Remote synchronization requires HTTPS unless an operator explicitly opts in
  to insecure HTTP for a specific command.
- Access files store SHA-256 token digests rather than plaintext bearer tokens.
- Restore checks the expected current file state, workspace containment, and
  snapshot integrity, creates a safety snapshot, and requires `--force` to
  override a conflict.

## Supported platforms

Release packaging targets:

- Windows amd64 and arm64.
- Linux amd64 and arm64.
- macOS amd64 and arm64.

## Known limitations

- Agent Audit Console is not an Endpoint Detection and Response system.
- It cannot capture Agent operations that do not pass through a supported CLI,
  MCP, or adapter integration boundary.
- Rollback cannot guarantee reversal of external side effects such as an
  already completed remote operation.
- A native Codex Sidebar or Audit Card is not part of v0.1.0.
- Token-protected browser login and automatic retention deletion are deferred.
- Exact snapshots may contain sensitive source content and must be protected by
  local filesystem permissions.

## License

Agent Audit Console is licensed under the Apache License 2.0. See `LICENSE` for
details.

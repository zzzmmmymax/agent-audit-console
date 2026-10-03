# Changelog

All notable changes to this project will be documented here.

## Unreleased - v0.2.0

### Added

- MCP v2 run/action lifecycles, automatic run contexts, correlation, discovery,
  policy explanation, approval status, rollback preview, bounded pagination,
  compact responses, stable errors, and idempotency.
- Agent Adapter v2 capability descriptors and normalized integration entry points.
- Safe `audit setup` flows for Codex, Claude Code, and Cursor, with `audit doctor`
  integration diagnostics.
- Event schema v2 and migration 3 while preserving v0.1 hash verification.
- Web Audit Experience v2 with server-filtered run search, action-grouped session
  timelines, parent/child trace context, bounded command/diff evidence, risk and
  approval investigation panels, live integrity verification, rollback preview,
  deep links, and 10,000-event pagination.

### Security

- Setup merges instead of overwriting, refuses malformed configuration, keeps
  bounded backups, and writes atomically.
- MCP cannot self-approve or execute restore; rollback remains a pending request.
- Web evidence is rendered as untrusted text; ANSI/control characters and large
  logs/diffs are bounded, and rollback preview does not expose object paths.

## v0.1.0 - 2026-10-03

### Added

- Local-first command, file, Git, and MCP audit capture.
- SQLite/WAL storage, canonical SHA-256 event chains, snapshots, and restore.
- Codex, Claude Code, Cursor, and custom agent adapters.
- YAML policy, approvals, redaction, JSON/HTML export, local Web console.
- Local RBAC, explicit remote sync, verification, diagnostics, and migrations.

### Security

- Conflict-aware restore with explicit force override and workspace/symlink
  containment.
- Full-chain verification against the trusted run head.
- Hashed API tokens, loopback-safe daemon defaults, HTTPS-first sync, bounded
  output capture, and expanded secret redaction.

### Known limitations

- Controlled CLI/MCP capture only; not OS-wide monitoring or EDR.
- External side effects are not generally reversible.
- Raw snapshots can contain secrets.
- Token-protected browser login is not included in v0.1.0.

### License

- Released under the Apache License 2.0.

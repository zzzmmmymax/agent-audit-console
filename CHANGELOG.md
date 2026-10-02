# Changelog

All notable changes to this project will be documented here.

## v0.1.0 - Unreleased

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
- The final project license remains an owner decision.

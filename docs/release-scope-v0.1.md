# v0.1.0 release scope

Agent Audit Console v0.1.0 is a local-first audit recorder for controlled AI
coding-agent activity. It is intended for individual developers and local
evaluation, not as an operating-system security product.

## Supported

- Local-first SQLite/WAL audit storage.
- Codex, Claude Code, Cursor, and custom agent adapters.
- `audit` CLI capture and inspection.
- MCP stdio tools for run/action recording and rollback requests.
- Embedded local Web console served by `auditd`.
- SHA-256 content-addressed file snapshots and conflict-aware file restore.
- Tamper-evident event hash chains and explicit run verification.
- YAML local and layered team policy, including approval gates.
- JSON and HTML audit export.
- Local bearer-token RBAC for HTTP APIs.
- Explicit, operator-invoked remote audit bundle synchronization.
- Windows, Linux, and macOS source builds on amd64 and arm64.

## Not supported or guaranteed

- Operating-system-wide process, filesystem, or network monitoring.
- Endpoint Detection and Response (EDR).
- Complete capture of operations that bypass the CLI/MCP integration boundary.
- Automatic rollback of every external side effect.
- Email recall, completed-payment reversal, or guaranteed rollback of actions
  already executed against remote systems.
- Protection against a local administrator replacing both the database and the
  trusted application binary and recomputing a complete chain.
- A native Codex Sidebar or Audit Card unless a stable official extension API
  becomes available.
- Automatic audit-data retention deletion in v0.1.0; zero retention means keep
  indefinitely and nonzero values are diagnostic/reserved configuration.
- Browser-based login for a token-protected Web console. Authenticated HTTP mode
  is intended for API clients in v0.1.0; the default local Web console remains
  loopback-only.

Raw snapshot content is intentionally exact so that restoration is possible.
Snapshots can therefore contain secrets even though persisted event evidence,
exports, Web data, and sync bundles are redacted.

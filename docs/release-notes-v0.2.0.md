# Agent Audit Console v0.2.0

v0.2.0 turns the local-first recorder into a more complete investigation and policy-preflight console while preserving v0.1 data and workflows.

## Highlights

- MCP v2 run contexts, action lifecycle, parent/child correlation, stable errors, pagination, and idempotency.
- Agent Adapter v2 plus safe `audit setup` workflows for Codex, Claude Code, and Cursor.
- Web Audit Experience v2 with session timelines, action chains, command/diff evidence, risk and approval panels, live integrity verification, and rollback preview.
- Policy simulation, semantic validation, regression tests, complete rule traces, canonical fingerprints, and recorded-versus-current explanation.
- A read-only Web Policy Inspector and MCP `simulate_policy` preflight.

## Agent integration

`audit setup <agent>` previews configuration changes by default and only writes with `--apply`. It parses, merges, backs up, and atomically updates configuration without removing other MCP servers.

## Web Audit Experience

The embedded console groups persisted events into action lifecycles, keeps 10,000-event runs paginated, and treats all evidence as untrusted text. Rollback remains a separately authorized request and execution workflow.

## Policy simulation

Use `audit policy validate`, `audit policy simulate`, `audit policy explain`, and `audit policy test`. Simulation is transient and never authorizes execution; the real operation is always evaluated again.

## Security and compatibility

v0.2.0 retains RBAC boundaries, approval and rollback separation, workspace/symlink containment, secret redaction, hash-chain verification, bounded inputs, RE2 matchers, and HTTPS-first explicit sync. Legacy policy aliases and v0.1 MCP/CLI contracts remain available.

## Upgrade from v0.1

Replace the binaries and reuse the existing data directory. Database migration is automatic. Existing runs, events, diffs, snapshots, and v0.1 hash chains remain readable and verifiable; no reset or reinitialization is required.

## Supported platforms

- Windows amd64 and arm64
- Linux amd64 and arm64
- macOS amd64 and arm64

Each archive contains `audit`, `auditd`, `agent-audit-mcp`, README, and LICENSE. Verify downloads with `checksums.txt`.

## Known limitations

- Not OS-level EDR; only integrated operations are captured.
- External effects may not be reversible.
- No native Codex sidebar, full historical policy snapshot diff, or Web policy editor.
- Static policy shadow detection is limited to identical matchers.
- Large diffs are truncated in the browser.
- A rollback request does not mean a restore was executed.

## License

Apache License 2.0.

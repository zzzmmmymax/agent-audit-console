# Stage 8 report

## Branch and baseline

- Branch: `feature/v0.2-mcp`
- Base Commit: `91aedca`
- Final Commit: branch tip containing this report (`git rev-parse HEAD`; a commit
  cannot embed its own SHA without changing that SHA)
- Scope: Stage 8 only. No Stage 9–11 Web/Policy Simulation/release work.
- Remote note: the initial `git pull --ff-only origin master` could not reach
  GitHub port 443. The local `master` and recorded `origin/master` both pointed
  at `91aedca` before the branch was created.

## MCP v2 tools and compatibility

The server now exposes 21 tools grouped into Run, Action, Query, Policy,
Approval, Rollback and Diagnostics. `start_run`, `record_action`,
`get_run_summary`, and `request_rollback` remain available under their v0.1
names and accept their documented v0.1 fields. `capabilities` returns server and
capability versions, tools, adapters, event kinds, policy decisions, stable
error codes, and approval/rollback support.

Run Context uses explicit run IDs or SHA-256-derived context keys. Session-aware
keys are stable within an MCP process; a start without a session receives a
unique returned context key so concurrent agents cannot overwrite each other.

Action Lifecycle supports planned, started, completed, failed, blocked and
cancelled states. Schema v2 events carry action, parent-action and correlation
IDs. Adapter v2 provides capability descriptors plus normalized agent,
session, tool-call, command and file-change entry points for Codex, Claude Code,
Cursor and custom agents.

## Integration setup and config safety

`audit setup codex|claude-code|cursor` previews by default and supports
`--dry-run`, `--print`, `--apply`, `--config-path` and `--mcp-command`.
Apply validates the target format, merges only `agent-audit`, preserves other
servers/settings, creates timestamped backups, keeps the newest three, and
performs an atomic replacement. Unknown/malformed JSON or TOML is refused.
`audit doctor` reports PASS/WARN/FAIL integration status without secrets.

The implemented paths follow public MCP integration surfaces: Codex user TOML,
Claude Code user JSON, and Cursor user MCP JSON. Nonstandard/future locations
must use `--config-path`; setup does not guess and write elsewhere.

## Security

- RBAC: unchanged HTTP role enforcement; MCP rollback can only create a pending
  request and cannot execute restore or weaken operator/admin API checks.
- Approval: MCP records a pending approval event and has no approve tool.
- Rollback: preview is read-only and reports hashes/conflicts; requests validate
  run ownership and remain subject to existing restore containment/integrity.
- Query safety: diff paths are constrained to the recorded workspace; evidence
  and diffs are bounded; internal SQL/filesystem errors are not returned.
- Local first: no cloud connection, upload, telemetry or background sync added.

## Pagination, idempotency and error model

`list_runs` and `list_events` use bounded limits (maximum 1,000), `has_more`
and cursors. Event evidence and diffs support compact/truncated responses.
Run start, action lifecycle writes and rollback requests persist idempotent
typed responses. Errors use the documented machine-readable code prefix while
retaining a concise human-readable message.

## Database migration and hash compatibility

Migration 3 adds event correlation/version columns, action records, run
contexts and idempotency keys. A pre-schema migration path upgrades a real
v0.1 table layout before new indexes are created. Tests open and upgrade the
legacy layout twice. Historical events remain schema 1 and use the exact v0.1
hash payload; schema 2 hashes the new lifecycle/correlation fields. Mixed v1/v2
chains are covered by verification tests.

## Tests, demo and frontend

- `go test ./...`: PASS
- `go vet ./...`: PASS
- `go mod verify`: PASS (`all modules verified`)
- `npm test`: PASS (2 tests)
- `npm run build`: PASS
- MCP lifecycle/context/idempotency/approval/compatibility/demo tests: PASS
- Setup JSON/TOML merge, backup and malformed-config tests: PASS
- Local `go test -race ./...`: NOT RUN; this Windows Go environment reports
  `-race requires cgo`. Linux CI remains the required race gate.
- Automated demo: `go test ./mcp -run TestMCPIntegrationDemo -v`

## Benchmarks

Windows/amd64, Intel i7-13620H, `-benchtime=100ms`:

| Operation | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| start_run | 1,171,272 | 3,663 | 76 |
| record_action | 3,598,396 | 14,411 | 289 |
| get_run_summary (100 events) | 7,704,363 | 1,004,139 | 12,378 |
| list_events 100 | 1,163,953 | 334,790 | 4,765 |
| list_events 1000 | 8,605,800 | 3,579,840 | 48,756 |

These are a Stage 8 baseline, not a cross-machine performance guarantee.

## Known issues

1. The race detector must be confirmed by Linux CI because local CGO is off.
2. Remote pull/push require GitHub connectivity; no history rewrite, merge,
   version tag or GitHub Release is part of Stage 8.
3. Automatic capture still observes only CLI/MCP/Adapter-controlled operations,
   not all activity by an agent or operating system.
4. MCP authorization is the local stdio process/OS trust boundary. Networked
   authenticated MCP transport is intentionally deferred; HTTP RBAC is unchanged.

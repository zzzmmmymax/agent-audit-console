# Agent Audit Console

Local-first audit, policy, and rollback tooling for AI coding agents.

[![CI](https://github.com/zzzmmmymax/agent-audit-console/actions/workflows/ci.yml/badge.svg)](https://github.com/zzzmmmymax/agent-audit-console/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/zzzmmmymax/agent-audit-console)](https://github.com/zzzmmmymax/agent-audit-console/releases/latest)
[![License](https://img.shields.io/github/license/zzzmmmymax/agent-audit-console)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go)](go.mod)

Agent Audit Console records and reviews operations performed through supported
CLI, MCP, and Agent Adapter integrations, including shell commands, file
changes, Git activity, approvals, and rollback evidence.

- **Current release:** [v0.1.0](https://github.com/zzzmmmymax/agent-audit-console/releases/tag/v0.1.0)
- **License:** [Apache License 2.0](LICENSE)
- **Platforms:** Windows, Linux, and macOS on amd64 and arm64

## Why Agent Audit Console?

AI coding agents can execute shell commands, modify files, use MCP tools, and
interact with Git. Agent Audit Console focuses on four questions:

- What happened?
- What changed?
- Was it allowed?
- Can local state be restored?

It is an Agent audit and control tool. It is **not** an EDR, antivirus,
full-operating-system monitor, sandbox, or replacement for endpoint security.

## Features in v0.1.0

- Local-first SQLite/WAL audit event storage.
- Shell command, file change, and Git operation tracking.
- Unified diffs and SHA-256 content-addressed pre-change snapshots.
- Conflict-aware controlled file restore with containment checks.
- Canonical event-integrity hash chains and `audit verify`.
- YAML policy rules, low/medium/high risk levels, and approval workflow.
- Secret redaction before event persistence, display, export, and sync.
- JSON and self-contained HTML audit exports.
- MCP server with run, action, summary, and rollback-request tools.
- Embedded local React Web Console.
- Codex, Claude Code, Cursor, and custom Agent Adapters.
- Viewer/operator/admin bearer-token RBAC with token digest storage.
- Explicit HTTPS remote audit synchronization with idempotency keys.
- `audit doctor` configuration and dependency diagnostics.

## Architecture

```text
Codex / Claude Code / Cursor / custom agent
                     |
              CLI / MCP / Adapter
                     |
                 Audit Core
        +------------+-------------+
        |            |             |
    Event Store  Snapshot Store  Policy Engine
        |            |             |
        +-------- Rollback --------+
                     |
                  HTTP API
                     |
             Local Web Console
```

Audit data remains local unless an operator explicitly invokes remote sync.
See [Architecture](docs/architecture.md), [Event Schema](docs/event-schema.md),
and the [v0.1.0 Release Scope](docs/release-scope-v0.1.md).

## Quick start

### A. Download a release

1. Download the archive for your operating system and architecture from the
   [v0.1.0 release](https://github.com/zzzmmmymax/agent-audit-console/releases/tag/v0.1.0).
2. Verify it against `checksums.txt` and extract the archive.
3. Put the extracted directory on `PATH`, or run the binaries from that
   directory.

Linux or macOS:

```shell
./audit version
./audit doctor
./audit run -- go test ./...
```

Windows PowerShell:

```powershell
.\audit.exe version
.\audit.exe doctor
.\audit.exe run -- go test ./...
```

Release binaries do not require Node.js. The command prints a run ID that can
be used for inspection, integrity verification, export, and restore.

### B. Build from source

Prerequisites: Go 1.26 or newer, Git, and Node.js 22 or newer only when
rebuilding the Web assets.

```shell
git clone https://github.com/zzzmmmymax/agent-audit-console.git
cd agent-audit-console
npm --prefix web ci
npm --prefix web run build
go build -o bin/audit ./cmd/audit
go build -o bin/auditd ./cmd/auditd
go build -o bin/agent-audit-mcp ./mcp
```

For a disposable end-to-end example, run `scripts/demo.sh` on Unix-like
systems or `scripts/demo.ps1` on Windows. The demo creates its own temporary
Git repository and does not modify this checkout.

## Inspect and verify a run

```shell
audit log
audit show RUN_ID
audit verify RUN_ID
```

- `audit log` lists recorded runs.
- `audit show` prints a run and its event timeline.
- `audit verify` checks the complete event hash chain against the trusted run
  head and reports insertion, deletion, modification, or broken links.

Export a portable bundle when needed:

```shell
audit export RUN_ID --format json --output audit.json
audit export RUN_ID --format html --output audit.html
```

## Restore a file

```shell
audit restore path/to/file
```

Restore targets the latest recorded pre-change state for one file. It checks
workspace containment, symlink and junction escapes, snapshot integrity, and
whether the current file changed since capture. `--force` can override only a
current-state conflict; it does not bypass containment or integrity checks.

Restore cannot undo every external side effect. API calls, payments, sent
messages, pushes, and other completed remote changes may require manual
compensation.

## Local Web Console

Start the daemon:

```shell
auditd
```

Open [http://127.0.0.1:8777](http://127.0.0.1:8777). The daemon binds to
loopback by default and refuses a non-loopback address without an access
policy. The console presents runs, timelines, commands, file diffs, risk and
approval decisions, integrity state, and rollback records.

## MCP integration

`agent-audit-mcp` is a stdio MCP server exposing these tools:

- Run: `start_run`, `get_run`, `get_run_summary`, `finish_run`
- Action: `plan_action`, `record_action`, `complete_action`, `fail_action`
- Query: `list_runs`, `list_events`, `get_event`, `get_file_diff`
- Policy/approval: `evaluate_action`, `explain_policy`, `request_approval`,
  `get_approval_status`
- Rollback: `preview_rollback`, `request_rollback`, `get_rollback_status`
- Diagnostics: `health`, `capabilities`

The original v0.1 tool names remain compatible. MCP v2 adds safe automatic run
contexts, correlation, compact responses, cursor pagination, typed errors, and
idempotency keys.

Generic MCP client configuration:

```json
{
  "mcpServers": {
    "agent-audit": {
      "command": "/absolute/path/to/agent-audit-mcp",
      "args": []
    }
  }
}
```

Use the `.exe` filename on Windows. Set `AGENT_AUDIT_HOME` or add
`--data-dir /path/to/data` to `args` when the MCP server and CLI must share a
non-default data directory. Apply the equivalent MCP server registration in
Codex, Claude Code, Cursor, or another MCP-compatible client.

### Agent integration setup (v0.2 development)

Setup is preview-only unless `--apply` is supplied:

```shell
audit setup codex --dry-run
audit setup claude-code --dry-run
audit setup cursor --dry-run
audit setup codex --apply
audit doctor
```

Setup parses, merges, validates, backs up, and atomically updates agent config
without removing other MCP servers. Use `--config-path` for a nonstandard
location. See the [Codex](docs/integrations/codex.md),
[Claude Code](docs/integrations/claude-code.md),
[Cursor](docs/integrations/cursor.md), and
[custom agent](docs/integrations/custom-agent.md) guides.

## Agent Adapters

The `AgentAdapter` boundary standardizes agent identity and event metadata for:

- `codex`
- `claude-code`
- `cursor`
- `custom`

Select an adapter when wrapping a command:

```shell
audit run --agent codex -- your-command
audit run --agent claude-code -- your-command
audit run --agent cursor -- your-command
audit run --agent custom -- your-command
```

These are product-neutral Adapter/CLI/MCP integrations, not claims of deep
native UI integration with those products.

## Policy and approvals

The first startup creates `policy.yaml`. The actual v0.1.0 rule schema matches
event kinds and command regular expressions and produces a risk and decision:

```yaml
version: 1
default:
  risk: low
  decision: allowed
rules:
  - id: git-push
    description: Publishing commits changes remote state.
    command_regex: '(?i)(^|\s)git(?:\.exe)?\s+push(?:\s|$)'
    risk: high
    decision: pending
```

High-risk pending commands require interactive confirmation or the explicit
`--approve-high-risk` automation flag. Team-policy files are evaluated before
local rules and merge toward the stricter result.

## Configuration

Resolution priority is **CLI flag > environment variable > YAML config file >
default**. Select a YAML file with `--config` or `AGENT_AUDIT_CONFIG`.

| Setting | Environment variable | YAML key | Default |
|---|---|---|---|
| Data directory | `AGENT_AUDIT_HOME` | `data_dir` | OS user config directory |
| SQLite path | `AGENT_AUDIT_DATABASE` | `database_path` | `<data>/audit.db` |
| Snapshot directory | `AGENT_AUDIT_SNAPSHOT_DIR` | `snapshot_dir` | `<data>/snapshots` |
| Local policy | `AGENT_AUDIT_POLICY_FILE` | `policy_file` | `<data>/policy.yaml` |
| Team policies | `AGENT_AUDIT_TEAM_POLICY` | `team_policies` | none |
| Web/API address | `AGENT_AUDIT_LISTEN_ADDRESS` | `listen_address` | `127.0.0.1:8777` |
| Access policy | `AGENT_AUDIT_ACCESS_FILE` | `access_file` | `<data>/access.yaml` |
| Sync endpoint | `AGENT_AUDIT_SYNC_ENDPOINT` | `sync_endpoint` | none |
| Sync token variable | `AGENT_AUDIT_SYNC_TOKEN_ENV` | `sync_token_env` | `AGENT_AUDIT_SYNC_TOKEN` |
| Retention days | `AGENT_AUDIT_RETENTION_DAYS` | `retention_days` | `0` (keep forever) |

Nonzero retention is reported by `audit doctor`; automatic evidence deletion
is intentionally disabled in v0.1.0.

## RBAC and API

Create a local API token and start a protected daemon:

```shell
audit access-token --name alice --role operator
auditd --access-file /path/to/access.yaml
```

The plaintext token is displayed once; only its SHA-256 digest is stored.
`viewer` reads audit data, `operator` can also submit rollback requests, and
`admin` is reserved for administrative capabilities. See the complete
[RBAC matrix](docs/rbac.md).

The v0.1.0 browser UI has no token-login flow. Protected mode targets API
clients; normal local Web use should remain loopback-only.

## Security model

- Storage and Web/API binding are local-first by default.
- Token files store SHA-256 digests, not plaintext bearer tokens.
- Evidence, exports, Web responses, and sync bundles apply secret redaction.
- Hash-chain verification detects changes to recorded event history.
- Remote sync requires HTTPS unless explicitly overridden for local testing.
- Restore verifies containment, current state, and snapshot integrity.
- Non-loopback daemon binding requires an access policy.

Raw snapshots preserve exact bytes and can contain sensitive source material.
Protect the data directory with operating-system permissions. See
[SECURITY.md](SECURITY.md) and the detailed [security model](docs/security.md).

## Known limitations

1. Agent Audit Console is not an operating-system EDR.
2. Operations outside supported CLI, MCP, or Adapter boundaries may not be
   recorded.
3. External side effects may not be automatically reversible.
4. A native Codex Sidebar or Audit Card is not part of v0.1.0.
5. Token-protected browser login and automatic retention deletion are deferred.
6. GitHub Actions may emit non-blocking Node.js runtime deprecation warnings
   for current official action versions.

v0.1.0 is the first public release. Its documented safety boundaries are
enforced and its release gates pass, but interfaces and storage migrations may
continue to evolve in later versions.

## Development

```shell
git clone https://github.com/zzzmmmymax/agent-audit-console.git
cd agent-audit-console
go mod verify
go test ./...
go vet ./...
npm --prefix web ci
npm --prefix web test
npm --prefix web run build
```

Linux CI additionally runs `go test -race ./...`. Build the three binaries with
`go build ./cmd/audit`, `go build ./cmd/auditd`, and
`go build -o agent-audit-mcp ./mcp`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) before opening a pull request or feature
request.

## Security

Do not report vulnerabilities in public issues. See [SECURITY.md](SECURITY.md)
for the private reporting process and scope.

## License

Agent Audit Console is licensed under the [Apache License 2.0](LICENSE).

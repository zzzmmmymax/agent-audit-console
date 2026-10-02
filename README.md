# Agent Audit Console

Agent Audit Console is a local-first flight recorder, diff review system, and
controlled recovery console for AI coding agents. It records operations routed
through its CLI or MCP integration without attempting to monitor every process
on the computer.

The v0.1.0 release candidate supports Codex, Claude Code, Cursor, and custom
agents through one product-neutral event model.

## Core capabilities

- Tamper-evident, canonical SHA-256 event chains.
- SQLite in WAL mode with versioned, in-place migrations.
- Command, file, Git, approval, MCP, and rollback events.
- Content-addressed file snapshots with deduplication and integrity checks.
- Conflict-aware, atomic file restore with workspace and symlink containment.
- YAML risk policy, layered team rules, and high-risk approval gates.
- Secret redaction before event persistence, Web display, export, and sync.
- Embedded React Web console with paginated event timelines and split diffs.
- Viewer/operator/admin bearer-token RBAC.
- Explicit HTTPS audit-bundle synchronization with stable idempotency keys.

## Architecture

```text
Codex / Claude Code / Cursor / custom agent
                 |
          CLI or MCP adapter
                 |
       capture -> policy -> event service
                           /           \
                     SQLite/WAL    snapshots
                           |
                     HTTP API + embedded Web
                           |
                 explicit remote sync only
```

Audit data remains local unless the operator explicitly runs `audit sync`.
See [architecture](docs/architecture.md), [event schema](docs/event-schema.md),
[release scope](docs/release-scope-v0.1.md), and [security model](docs/security.md).

## Requirements and installation

- Go 1.26 or newer.
- Node.js 22 or newer only when rebuilding the Web assets.
- Git for the demo.

Build from source:

```shell
git clone https://github.com/agent-audit-console/agent-audit-console.git
cd agent-audit-console
npm --prefix web ci
npm --prefix web run build
go build -o bin/audit ./cmd/audit
go build -o bin/auditd ./cmd/auditd
go build -o bin/agent-audit-mcp ./mcp
```

Release archives are prepared for Windows, Linux, and macOS on amd64 and arm64.
`auditd` embeds the production Web assets; end users do not need Node.js.

## Quick start

From any project directory:

```shell
# Execute a command inside the audit boundary.
audit run -- go test ./...

# Copy the run ID printed above, then inspect and verify it.
audit log
audit show RUN_ID
audit verify RUN_ID

# Start the local Web console.
auditd
```

Open [http://127.0.0.1:8777](http://127.0.0.1:8777). The daemon binds to
loopback by default. It refuses a non-loopback address unless an access policy
is configured.

## CLI

```text
audit version
audit run [--agent codex|claude-code|cursor|custom] -- COMMAND [ARG...]
audit log
audit show RUN_ID
audit verify RUN_ID
audit restore [--force] FILE
audit export RUN_ID --format json|html --output FILE
audit access-token --name NAME --role viewer|operator|admin
audit sync RUN_ID --endpoint https://audit.example.com
audit doctor
```

`restore` affects exactly one recorded path. It refuses paths outside the
recorded workspace, symlink/junction escapes, and files changed since capture.
`--force` bypasses only the current-hash conflict; it does not bypass workspace
containment or snapshot verification. Restore itself creates snapshots,
rollback records, and rollback audit events.

Run `audit COMMAND --help` for every flag and command description.

## MCP

The release binary is named `agent-audit-mcp` and communicates over stdio. It
exposes:

- `start_run`
- `record_action`
- `get_run_summary`
- `request_rollback`

Example client configuration:

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

Set `AGENT_AUDIT_HOME` or pass `--data-dir` if the MCP server and CLI should
share a non-default data directory.

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
| Web/API listen address | `AGENT_AUDIT_LISTEN_ADDRESS` | `listen_address` | `127.0.0.1:8777` |
| Access policy | `AGENT_AUDIT_ACCESS_FILE` | `access_file` | `<data>/access.yaml` |
| Sync endpoint | `AGENT_AUDIT_SYNC_ENDPOINT` | `sync_endpoint` | none |
| Sync token variable name | `AGENT_AUDIT_SYNC_TOKEN_ENV` | `sync_token_env` | `AGENT_AUDIT_SYNC_TOKEN` |
| Retention days | `AGENT_AUDIT_RETENTION_DAYS` | `retention_days` | `0` (keep forever) |

Example configuration:

```yaml
data_dir: /var/lib/agent-audit
listen_address: 127.0.0.1:8777
team_policies:
  - /etc/agent-audit/team-policy.yaml
sync_endpoint: https://audit.example.com
sync_token_env: AGENT_AUDIT_SYNC_TOKEN
retention_days: 0
```

Nonzero retention is reported by `audit doctor` but automatic evidence deletion
is intentionally disabled in v0.1.0.

## Policy and approvals

The first startup creates `policy.yaml`. Rules can match event kinds and command
regular expressions, assign low/medium/high risk, and allow, reject, or place an
action into pending approval. Team-policy files are evaluated before local
rules; unmatched defaults merge to the stricter result.

High-risk pending commands require interactive confirmation or the explicit
`--approve-high-risk` automation flag.

## RBAC and API

Create a token and start a protected daemon:

```shell
audit access-token --name alice --role operator
auditd --access-file /path/to/access.yaml
```

The plaintext token is displayed once; only its SHA-256 digest is stored. Send
it as `Authorization: Bearer TOKEN`. Viewer can read, operator can also submit
rollback requests, and admin is reserved for administrative capabilities. The
v0.1.0 HTTP API has no token-management or policy-mutation route. See the full
[RBAC matrix](docs/rbac.md).

The browser UI does not implement a token login in v0.1.0. Protected mode is
intended for API clients; keep the normal local Web console loopback-only.

## Explicit remote sync

```shell
export AGENT_AUDIT_SYNC_TOKEN='replace-with-real-token'
audit sync RUN_ID --endpoint https://audit.example.com
```

PowerShell uses `$env:AGENT_AUDIT_SYNC_TOKEN = '...'`. Tokens are sent only in
the Authorization header, never in URLs. Sync verifies the complete chain,
requires HTTPS, sends export schema version 1, and uses `run_id + content_digest`
as the idempotency key. `--allow-insecure` exists only for explicit local
development.

## Security model and limitations

- This is not EDR or OS-wide monitoring.
- Actions that bypass the CLI/MCP boundary are not guaranteed to be captured.
- Payments, email, pushes, and other completed remote side effects are not
  guaranteed reversible.
- Hash chains are tamper-evident, not a defense against a local administrator
  replacing the database, binary, and trusted chain head together.
- Event evidence is redacted. Raw snapshots remain exact recovery artifacts and
  may contain secrets; protect the data directory.
- A native Codex Sidebar is not included without a stable official extension
  API.
- The repository currently carries a license-pending notice, not an open-source
  license grant.

## Build

Build the three standalone binaries without requiring Node.js at runtime:

```shell
go build ./cmd/audit
go build ./cmd/auditd
go build -o agent-audit-mcp ./mcp
```

The Web console is embedded in `auditd`. Rebuilding its source assets requires
Node.js during development, but users of a packaged binary do not need Node.js,
Vite, or npm.

## Test

```shell
go mod verify
go test ./...
go vet ./...
go test -race ./...       # requires CGO and a supported native toolchain
npm --prefix web ci
npm --prefix web test
npm --prefix web run build
```

Run the isolated demo with `scripts/demo.sh` on Unix-like systems or
`scripts/demo.ps1` on Windows. It creates a temporary Git repository and never
modifies the current checkout.

## Release

CI is configured for Windows, Ubuntu, and macOS. Release packaging is described
by `.goreleaser.yaml` and produces Windows ZIP files, Linux/macOS tarballs, and
SHA-256 checksums for `audit`, `auditd`, and `agent-audit-mcp`. Archives include
the README and the current license-status notice. No tag, release, or package is
created by this repository state. The final license must be selected by the
project owner before a public release.

See [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and
[CHANGELOG.md](CHANGELOG.md).

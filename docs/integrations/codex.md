# Codex integration

## Requirements

Install `audit` and `agent-audit-mcp`. Codex officially supports registering a
stdio MCP server with `codex mcp add`; its CLI and IDE extension share
`~/.codex/config.toml`.

## Setup

```shell
audit setup codex --dry-run
audit setup codex --apply
```

The command merges only `[mcp_servers.agent-audit]`, preserves other TOML
content, creates a timestamped backup, and installs the new file atomically.
Use `--config-path` when your Codex home is nonstandard and `--mcp-command` when
the executable is not beside `audit`.

## Verify and usage

Run `codex mcp list`, then `audit doctor`. Start with MCP `health` and
`capabilities`; create a run before recording actions.

## Troubleshooting and limitations

Malformed or unrecognized configuration is refused. Setup does not install or
start Codex and does not use private Codex APIs. It configures the documented
MCP boundary only.

# Cursor integration

## Requirements

Install Cursor plus `agent-audit-mcp`. Cursor reads MCP server definitions from
JSON configuration, commonly `~/.cursor/mcp.json`.

## Setup

```shell
audit setup cursor --dry-run
audit setup cursor --apply
```

Use `--config-path` if your Cursor build or policy uses another location. The
update merges `mcpServers.agent-audit`, backs up the old file, validates JSON,
and replaces atomically.

## Verify and usage

Restart or reload Cursor, inspect its MCP settings, and run `audit doctor`.
Call `health` and `capabilities` before beginning a run.

## Troubleshooting and limitations

Setup configures standard MCP only. It does not claim native Cursor file hooks;
events outside the MCP/CLI/adapter boundary are not captured.

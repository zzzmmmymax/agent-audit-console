# Claude Code integration

## Requirements

Install Claude Code plus `agent-audit-mcp`. Claude Code supports stdio MCP and
user-scope servers in `~/.claude.json`.

## Setup

```shell
audit setup claude-code --dry-run
audit setup claude-code --apply
```

The JSON merge preserves every other top-level setting and MCP server. Apply
creates a backup and uses an atomic replacement. `--config-path` can target a
project `.mcp.json` instead.

## Verify and usage

Run `claude mcp get agent-audit`, `claude mcp list`, or `/mcp`, followed by
`audit doctor`. Project-scoped MCP files may require interactive trust approval.

## Troubleshooting and limitations

Malformed JSON and a non-object `mcpServers` value are refused. Agent Audit
does not alter Claude permission settings or auto-approve its MCP server.

# Custom agent integration

## Requirements and setup

Launch `agent-audit-mcp` as a local stdio MCP server. Point `--data-dir` at the
same directory used by `audit` when CLI and MCP must share runs.

## Recommended lifecycle

1. Call `health`, then `capabilities`.
2. Call `start_run` with agent/session/workspace metadata.
3. Use `plan_action`, `record_action`, and a completion tool with stable
   `action_id`, `correlation_id`, and idempotency keys.
4. Call `finish_run`, `get_run_summary`, and `audit verify RUN_ID`.

## Troubleshooting and limitations

Provide an explicit `context_key` when one process multiplexes concurrent
sessions. Keep payloads small and page event queries. Approval and rollback
execution require an authorized human/API path; the MCP tools cannot self-
approve or directly restore files.

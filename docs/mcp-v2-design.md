# MCP v2 design

## Goals and compatibility

MCP v2 makes the local audit boundary discoverable and usable without clients
managing database details or manually threading a run ID through every call.
The v0.1 tools `start_run`, `record_action`, `get_run_summary`, and
`request_rollback` remain available. Their existing required fields continue to
work; v2 adds optional context, correlation, compact and idempotency fields.

The server advertises `protocol_version: "2.0"` and
`audit_capabilities_version: "2"`. This is an application capability version,
not a claim to replace the negotiated MCP transport protocol version.

## Tool groups

- Run lifecycle: `start_run`, `get_run`, `get_run_summary`, `finish_run`
- Action lifecycle: `plan_action`, `record_action`, `complete_action`, `fail_action`
- Audit query: `list_runs`, `list_events`, `get_event`, `get_file_diff`
- Policy: `evaluate_action`, `explain_policy`
- Approval: `request_approval`, `get_approval_status`
- Rollback: `preview_rollback`, `request_rollback`, `get_rollback_status`
- Diagnostics: `health`, `capabilities`

All list operations use bounded `limit` values and opaque/string cursors.
Compact mode replaces large evidence with a truncation marker. File diffs are
bounded to 64 KiB (4 KiB in compact mode).

## Run context

Every tool accepts an explicit `run_id`. A client may instead provide a
`context_key`, or a tuple of agent, session and workspace attributes. Context
keys are SHA-256-derived and stored locally. The automatic key also includes
the MCP server process ID, preventing unrelated concurrent server processes
from sharing a run accidentally. Clients that multiplex sessions through one
process should provide `session_id` or `context_key`.

## Event and action compatibility

Database migration 3 adds action records, run contexts, idempotency responses,
and event correlation columns. Historical rows default to event schema 1 and
continue to use the exact v0.1 canonical hash payload. New adapter events use
schema 2, whose hash includes `parent_action_id`, `correlation_id`, and
`action_status`. No historical event is rewritten.

Action states are `planned`, `started`, `completed`, `failed`, `blocked`, and
`cancelled`. Run states are typed constants for `created`, `running`,
`completed`, `failed`, and `cancelled`.

## Security and errors

MCP can request approval but cannot approve. MCP rollback creates a pending
request and never performs restore. Execution remains behind the existing
authorized API/CLI and restore checks. Paths passed to diff retrieval must stay
inside the recorded workspace. Responses redact evidence and do not expose
database errors, storage paths, tokens, or policy file paths.

Tool errors use stable prefixes: `INVALID_ARGUMENT`, `NOT_FOUND`, `CONFLICT`,
`POLICY_DENIED`, `APPROVAL_REQUIRED`, `UNAUTHORIZED`, `INTEGRITY_ERROR`, and
`INTERNAL`. The current implementation emits the subset applicable to exposed
operations while reserving the full vocabulary.

Idempotency keys are supported by run creation, lifecycle writes, and rollback
requests. A retry receives the previously stored typed response and does not
append another event or request.

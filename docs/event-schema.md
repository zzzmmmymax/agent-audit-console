# Event schema

## Event envelope

Every captured operation is represented by one immutable event.

| Field | Type | Meaning |
|---|---|---|
| `event_id` | string | Globally unique event identity |
| `run_id` | string | Agent task/run identity and hash-chain partition |
| `action_id` | string | Logical action shared by related events |
| `timestamp` | RFC 3339 timestamp | When the observed action occurred |
| `sequence` | unsigned integer | Contiguous ordering within one run |
| `actor` | object | Product-neutral actor type/id plus optional metadata |
| `kind` | enum | `command`, `file_change`, `git`, `mcp_call`, `approval`, or `rollback` |
| `intent` | string | Stated purpose of the action |
| `evidence` | object | Kind-specific JSON and references to large local artifacts |
| `risk` | object | Severity and reasons |
| `policy_decision` | object | Decision status, rule, and explanation |
| `reversibility` | object | Recovery status and limitations |
| `previous_hash` | SHA-256 hex | Prior event hash; empty only for a run's genesis event |
| `integrity_hash` | SHA-256 hex | Digest of this event, including `previous_hash` |

Timestamps are normalized to UTC with nanosecond precision when hashed. Map keys
are deterministically ordered by Go's JSON encoder. Embedded evidence JSON is
compacted by that encoder but otherwise retains its supplied structure; changing
its stored representation after sealing invalidates the event.

## Hash construction

`integrity_hash` is lowercase hexadecimal SHA-256 over a JSON object with fixed
field order containing every event field except `integrity_hash` itself.
`previous_hash` is included. Consequently, changing one event invalidates that
event and every successor link.

The hash chain is tamper-evident, not tamper-proof. A party with write access to
the entire local store can rebuild the chain. Signed run heads or external
anchors may be added later when stronger non-repudiation is required.

## Ordering rules

- Sequences are unique and contiguous within a run.
- All events in a verified chain must have the same `run_id`.
- An event's `previous_hash` must equal its immediate predecessor's
  `integrity_hash`.
- Append and run-head advancement must occur in one database transaction.

## SQLite mapping

Structured values are stored as validated JSON text. Core identifiers, kind,
sequence, timestamps, predecessor, and integrity hash remain first-class columns
for efficient filtering and constraint enforcement. The authoritative DDL is
embedded from `internal/events/schema.sql`.

## Evolution

Kind-specific evidence schemas will carry explicit schema versions when first
implemented. New event kinds require both a code constant and a schema migration;
unknown kinds must not silently pass validation. Existing events are never
updated in place.

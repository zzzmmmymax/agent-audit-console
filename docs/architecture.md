# Architecture

## Scope

The completed Stage 3 architecture keeps the stable domain and storage
boundaries local-first while allowing explicit product adapters, shared policy,
authenticated API access, and optional remote delivery.

The trusted capture boundary is deliberately narrow: operations launched by the
`audit` CLI or explicitly recorded through MCP/SDK integration. The project is
not an OS-wide process monitor or EDR.

## Components

```text
Codex / Claude Code / Cursor / custom agent
       |
       +-- registered adapter --- CLI or MCP --- capture contracts
                                      |
                                      v
                                event service
                                 /    |    \
                          policy   SQLite   snapshots
                                      |
                                  local API
                                      |
                                  local Web UI
                                      |
                           explicit verified sync (optional)
```

- `internal/events` owns the product-neutral event vocabulary, integrity chain,
  query contract, and SQLite schema.
- `internal/adapters` owns the `AgentAdapter` contract and converts vendor
  identities/actions into the stable event vocabulary.
- `internal/capture` implements controlled shell, file, and Git collection and
  translates observations into the stable event sink.
- `internal/snapshots` implements immutable content-addressed storage. It
  calculates SHA-256 while streaming, syncs and atomically installs objects,
  verifies content before restore, and deduplicates identical bytes.
- `internal/policy` separates risk decisions from capture. Shared YAML rules are
  evaluated before the local policy so a local first-match cannot weaken a
  matching team restriction; unmatched defaults are merged to the strictest
  risk and decision. The boundary remains compatible with OPA/Rego.
- `internal/rollback` separates plan construction from execution so every
  recovery action can be reviewed and audited.
- `internal/api`, `mcp`, and `web` are delivery adapters and do not own domain
  rules.
- `internal/auth` stores only token digests and assigns viewer, operator, or
  admin roles. `internal/syncclient` performs explicit integrity-gated remote
  delivery; neither package changes the event domain.

## Write path and transaction boundary

The event service is the only component permitted to append events:

1. validate the unsealed event;
2. begin an immediate SQLite transaction for the run;
3. read `runs.next_sequence` and `runs.head_hash`;
4. assign the next sequence and previous hash;
5. calculate the current integrity hash;
6. insert the immutable event;
7. atomically advance the run's sequence and head hash;
8. commit.

This design prevents two writers from producing competing successors. Database
constraints additionally enforce one event per `(run_id, sequence)`.

## Data ownership

SQLite stores event metadata, JSON evidence, snapshot metadata, and rollback
records. Snapshot bytes belong in a separate SHA-256 content-addressed object
directory; the database stores their hash and object path. Duplicate contents
therefore share one object while retaining distinct logical snapshot records.

WAL improves local reader/writer concurrency. `foreign_keys=ON` protects links,
and `synchronous=FULL` favors audit durability over write throughput.
`schema_migrations` records compatible in-place changes; startup is idempotent
and v0.1.x upgrades do not require deleting the database.

## Dependency direction

Domain packages expose interfaces. Capture and delivery adapters depend on the
domain, never the reverse. Agent-specific integrations translate their payloads
into core events and keep vendor fields inside metadata/evidence.

## Stage boundaries

- Stage 0: schemas, contracts, hash chain, tests, and design documents.
- Stage 1: CLI capture, file diff/snapshots/restore, SQLite repository, local
  HTTP/React UI, and MCP server.
- Stage 2: YAML risk rules, approvals, redaction, and export.
- Stage 3: agent adapters, shared policy, remote sync, and permissions.

All four planned stages are now represented. Remote sync stays disabled unless
an operator invokes `audit sync`; local data remains the system of record.

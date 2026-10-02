# Security model

## Security goals

- Keep audit records and snapshots local by default.
- Detect accidental or unauthorized modification of individual event records.
- Make capture and recovery boundaries explicit.
- Preserve enough verified pre-change content for reliable local restoration.
- Avoid turning audit storage into a secret collection system.

## Trust boundary and limitations

The audit console observes actions routed through its CLI, MCP server, or future
SDK adapters. It does not monitor every process, kernel event, filesystem access,
or network request. An agent that bypasses these integration points creates a
visibility gap. Product surfaces must describe coverage as “controlled activity,”
not “all activity.”

The SHA-256 chain detects record edits, removal, reordering, and broken links
when a trusted head or complete chain is available. It does not stop a local
administrator or attacker with full storage access from replacing the database
and recomputing every hash. Future signing or external anchoring can strengthen
that guarantee without changing the event model.

## Local storage

- The database enables WAL, foreign keys, and full synchronization.
- Runtime files should be created with user-only permissions where supported.
- HTTP binds to loopback by default. Supplying `auditd --access-file` requires a
  bearer token for API routes while leaving the health probe public.
- Snapshot installation must use a temporary file, hash while streaming, fsync,
  atomic rename, and post-write verification.
- Restore must reject paths outside the enrolled workspace and must not follow
  symlinks across that boundary.

## Secrets and privacy

Stage 2 applies mandatory redaction before persistence to command arguments,
stdout/stderr, textual diffs, run command metadata, and MCP evidence. Built-in
patterns cover tokens, passwords, API keys, secrets, Bearer authorization, and
common database connection strings. Raw snapshots remain exact recovery
artifacts and must therefore be treated as sensitive local data.

Secrets must never be used as identifiers, snapshot paths, or hash metadata.
Hashing a secret does not make it safe when the input space is guessable.

High-risk YAML matches enter `pending` before execution. Local confirmation
creates an immutable `approved` event; refusal or unavailable input creates a
`rejected` event and cancels the run before process launch.

Shared policy paths from `AGENT_AUDIT_TEAM_POLICY` are evaluated before local
rules. This precedence prevents a local first-match rule from weakening the
same operation, though filesystem administrators can still replace local
configuration and binaries.

## API access and remote sync

Access files contain token names, roles, and SHA-256 token digests, never the
plaintext bearer token. The token is printed once when created. Viewer,
operator, and admin form an ordered role model; current read-only API routes
require viewer access and future mutation routes can require stronger roles.

Remote synchronization never runs in the background. `audit sync` verifies the
entire event chain before sending a bundle, reads the bearer token from an
environment variable, requires HTTPS by default, and supplies an idempotency
key derived from the run ID and trusted head hash. A remote receiver is outside
the local trust boundary and must independently authenticate, authorize, and
retain the bundle securely.

## Recovery safety

Rollback planning and execution are separate contracts. Restore verifies the
snapshot hash and current file hash, confirms lexical and resolved workspace
containment, rejects symlink targets, creates a safety snapshot, performs an
atomic replacement/removal, and appends a rollback event. Conflicts require an
explicit `--force`, which never bypasses containment or integrity checks.
External side effects such as pushes, payments, or messages are not advertised
as automatically reversible.

## Database integrity

The application must not allow updates or deletions through its normal event
repository. SQLite transactions protect append plus run-head movement. Startup
applies versioned migrations. Verification reads a consistent database
snapshot, checks every sequence/link/content hash, compares the tail with the
trusted run head, and surfaces the first broken event.

# v0.1.0 release audit

> Historical status note: RA-025 was resolved when the project owner selected
> Apache-2.0 before the v0.1.0 release.

Audit date: 2026-10-02 (Asia/Shanghai)

This document records the repository state observed before Stage 4 changes.
Severity is based on impact to a public v0.1.0 release. “Required” means the
issue blocks the release gate defined for this project.

## Repository baseline

- Git branch: `master`.
- Git history: no commits yet; every project file is untracked.
- CI: absent.
- Go module: `github.com/agent-audit-console/agent-audit-console`, Go 1.26.
- Frontend: React/Vite with a lockfile, but dependencies are declared as
  `latest` and there is no test script.
- License, contribution guide, security policy, changelog, GoReleaser config,
  demo, release scope, and RBAC matrix: absent.
- Baseline `go test ./...`, `go vet ./...`, and frontend production build:
  PASSED.
- Baseline `go test -race ./...`: NOT RUN successfully because this Windows
  environment has `CGO_ENABLED=0` and the race detector requires cgo.
- Baseline frontend tests: FAILED because no `test` script exists.
- npm vulnerability query: NOT VERIFIED because the configured npm mirror
  returns `NOT_IMPLEMENTED` for the audit API.

## Findings

| ID | Severity | Finding | Release fix required | Planned remediation |
|---|---|---|---|---|
| RA-001 | Critical | Restore has no expected-current-hash conflict check, so a user edit made after capture can be silently overwritten. | Yes | Persist expected post-run state, refuse conflicts by default, and add explicit `--force`. |
| RA-002 | High | Restore containment is lexical only. Existing symlinks/junctions in the target parent chain can escape the enrolled workspace. | Yes | Resolve the workspace and nearest existing parent, reject symlink/reparse traversal, and test escape attempts. |
| RA-003 | High | Deleting a target during restore uses direct removal rather than a recoverable atomic exchange. | Yes | Move the target to a same-directory backup, record the operation, and restore the backup on failure. |
| RA-004 | High | Run verification reads at most 10,000 events and does not compare the calculated tail with `runs.head_hash`; tail deletion can be missed. | Yes | Verify in pages, validate every link, and compare count, next sequence, and trusted run head. Return the first failing event. |
| RA-005 | High | Hashing includes raw evidence JSON representation. Semantically equal objects with different key order are not canonical. | Yes | Hash a canonical evidence digest while preserving a compatibility path for existing chains created by the current writer. |
| RA-006 | High | SQLite has no schema version or migration mechanism and no `busy_timeout`. | Yes | Add idempotent schema migrations, version introspection, busy timeout, restart/migration/concurrency/corruption tests. |
| RA-007 | High | API RBAC is only applied to read routes; no operator rollback-request route or route permission matrix exists. | Yes | Add an operator-only rollback request endpoint, retain viewer-only reads, document and test the matrix. |
| RA-008 | High | `auditd` can be bound publicly without authentication if the operator supplies a non-loopback address. | Yes | Refuse non-loopback binding without an access policy and keep `127.0.0.1` as the default. |
| RA-009 | High | Web run detail returns and renders up to 10,000 event cards at once. | Yes | Add API cursor pagination and incremental UI loading with bounded page sizes. |
| RA-010 | High | Snapshot objects are renamed without syncing file content first; concurrent identical writes can fail instead of deduplicating. | Yes | fsync before install and tolerate a verified winner of a concurrent content-addressed write. |
| RA-011 | Medium | Snapshot hash input validation checks length but not lowercase hexadecimal. | Yes | Strictly validate SHA-256 hexadecimal before deriving object paths. |
| RA-012 | High | Redaction misses common GitHub, OpenAI, AWS access keys and private-key headers; approval prompts can print the unredacted command. | Yes | Expand patterns, redact approval display, and add persistence/export/sync regression tests. |
| RA-013 | Medium | Command stdout and stderr are buffered without a limit, allowing unbounded memory use. | Yes | Add a bounded capture writer and an explicit truncation marker while preserving live output. |
| RA-014 | Medium | Sync idempotency uses run ID plus head hash, which does not cover mutable export fields outside the event chain. | Yes | Calculate a stable export-content digest and use `run_id + export_digest`; rename the insecure flag to `--allow-insecure`. |
| RA-015 | Medium | API 500 responses expose raw internal errors and there is no CORS policy declaration. | Yes | Return generic internal errors, preserve useful 4xx errors, and explicitly use same-origin/no permissive CORS headers. |
| RA-016 | Medium | `/readyz` is absent and `/healthz` does not distinguish liveness from dependency readiness. | Yes | Add readiness checks for SQLite, snapshot storage, and policy sources without returning sensitive paths. |
| RA-017 | Medium | CLI has no `version`, `verify`, or `doctor`; restore help does not explain overwrite scope. | Yes | Add the commands, ldflags version metadata, detailed integrity output, diagnostics, and consistent errors. |
| RA-018 | Medium | Configuration is spread across flags and two environment variables, with no documented config-file priority. | Yes | Add a typed configuration layer and document flag > environment > YAML config > default behavior. |
| RA-019 | Medium | The frontend has no test runner and no explicit integrity `Unknown` or rollback availability states. | Yes | Add a small Vitest suite and explicit Verified/Broken/Unknown plus Available/Partial/Unavailable presentation. |
| RA-020 | Medium | `events.NewID` panics if the OS random source fails. | Yes | Replace the panic with a collision-resistant non-panicking fallback. |
| RA-021 | Medium | JSON marshal errors are ignored in several event and rollback paths. | Yes | Propagate serialization failures where inputs are not statically guaranteed. |
| RA-022 | Medium | Export writes directly to the destination and can leave a partial/truncated file on failure. | Yes | Write to a sibling temporary file, sync, close, and atomically rename. |
| RA-023 | Medium | No multi-platform CI or reproducible release automation exists. | Yes | Add Windows/Ubuntu/macOS CI and GoReleaser archives/checksums for six target combinations. |
| RA-024 | Low | Architecture documentation still describes implemented snapshot work in future tense. | Yes | Correct documentation as part of release documentation consolidation. |
| RA-025 | Medium | There is no license decision. Public source publication would grant no reuse rights. | Owner decision | Add a license-pending notice and report this clearly; do not choose an OSS/commercial license for the owner. |
| RA-026 | Medium | Linux and macOS runtime behavior has not been executed locally. | CI required | Cross-compile locally and add CI runners; report local platform status accurately as NOT VERIFIED until CI runs. |
| RA-027 | Low | The local Web UI cannot supply a bearer token when API authentication is enabled. | No for local-only v0.1.0 | Document that authenticated API mode targets API clients; a browser credential flow is deferred rather than weakening token handling. |
| RA-028 | Low | Automatic retention deletion is not implemented. | No | Define `retention_days` with zero meaning keep forever; expose it in config/doctor but do not silently delete audit evidence in v0.1.0. |

## Confirmed strengths

- Local-first storage and loopback default are already present.
- SQLite WAL, foreign keys, strict tables, and full synchronization are enabled.
- Event append and run-head update are transactional, and sequence uniqueness is
  enforced by SQLite.
- Snapshot objects are content-addressed and verified before restore.
- Normal API routes are read-only and protected when an access file is enabled.
- Remote synchronization is explicit, uses the Authorization header, rejects
  plaintext HTTP by default, and validates the chain before sending.
- MCP action evidence is redacted before persistence.
- File paths use `filepath` in the core implementation; commands are executed
  directly without assuming PowerShell, cmd, sh, or bash.

## Release decision at audit start

**NOT RELEASE-READY.** RA-001 through RA-024 contain release-gate work. RA-025
requires an owner licensing decision for open-source reuse. Stage 4 will apply
small, testable fixes and will not push, tag, publish, merge, or contact a real
sync server.

# Stage 4 completion report

> Historical status note: the license blocker recorded in this Stage 4 report
> was resolved when the project owner selected Apache-2.0 before v0.1.0.

Report date: 2026-10-02 (Asia/Shanghai)

## 1. Completion status

Stage 4 release-hardening implementation is complete. The codebase is a
v0.1.0 release candidate, but the public release gate is **not yet cleared**:
the project owner must choose a license and the newly added CI workflow must
complete successfully on all hosted platforms before a tag or release is
created.

No push, merge, tag, release, package publication, upload, or real remote sync
was performed.

## 2. Release audit findings

The pre-change audit is recorded in [release-audit.md](release-audit.md). It
identified 28 findings: restore conflict and containment risks, incomplete hash
verification, missing database migrations, incomplete redaction, unbounded
command capture, API/RBAC and readiness gaps, frontend scalability, absent CI
and release automation, and documentation/release-governance gaps.

RA-001 through RA-024 were addressed in code, tests, configuration, automation,
or documentation. RA-025 requires an owner licensing decision. RA-026 is
partially verified by cross-compilation; Linux and macOS runtime behavior awaits
CI. RA-027 and RA-028 are explicitly documented v0.1.0 limitations.

## 3. Implemented fixes

- Added conflict-aware restore with expected-current-state checks and explicit
  `--force` override.
- Added lexical and resolved-path workspace containment, including symlink
  escape rejection, safety snapshots, atomic replacement/deletion, and recovery
  attempts if audit recording fails.
- Added snapshot hash validation, fsync-before-install, integrity checks, and
  safe concurrent content-addressed deduplication.
- Changed full-run integrity verification to paginate every event and validate
  sequence, previous hash, event hash, event count, next sequence, and trusted
  run head. The first failing event is reported.
- Canonicalized evidence JSON for new event hashes while preserving verification
  compatibility with development-era chains.
- Added versioned, idempotent SQLite migrations, busy timeout, WAL readiness,
  and corruption/concurrency/restart tests.
- Added typed YAML/environment/CLI configuration with precedence:
  CLI flag > environment > YAML file > default.
- Added `audit version`, `audit verify`, and `audit doctor`; normalized command
  errors and exit codes; made export writes atomic.
- Added bounded stdout/stderr capture with an explicit truncation marker.
- Expanded secret redaction for authorization headers, common provider keys,
  private keys, structured secrets, and database connection strings.
- Hardened sync to require HTTPS by default, reject URL credentials/query data,
  use authorization headers only, redact remote errors, and calculate a stable
  content-digest idempotency key.
- Added API cursor pagination, generic internal errors, security headers,
  liveness/readiness endpoints, RBAC, and an operator rollback-request route.
- Refused non-loopback `auditd` binding unless an access policy is configured.
- Added bounded Web timeline pagination and explicit integrity/rollback states.
- Added frontend unit tests and pinned dependency versions.
- Added MCP configuration/version integration and validated, redacted rollback
  requests.
- Added CI, GoReleaser configuration, checksums/archives, contributor/security
  documentation, changelog, RBAC matrix, release scope, and demos.

## 4. Remaining release work

| Item | Status | Required action |
|---|---|---|
| License selection | BLOCKING / OWNER ACTION | Replace the license-pending notice with the owner's chosen license before public open-source release. |
| Hosted CI | NOT RUN | Commit the repository and run `.github/workflows/ci.yml`; require all Windows, Ubuntu, and macOS jobs to pass. |
| Local race detector | NOT VERIFIED | This Windows host has no GCC; rely on the configured Ubuntu CGO race job or install a supported C toolchain. |
| Linux/macOS runtime | NOT VERIFIED LOCALLY | Cross-build passed; CI runners must exercise tests and builds. |
| npm vulnerability audit | NOT VERIFIED | The configured npm mirror does not implement the audit API; run against a registry that supports it. |
| GoReleaser schema check | NOT RUN | GoReleaser is not installed locally; run `goreleaser check` in a trusted release environment. |
| Token-protected browser login | DEFERRED | v0.1.0 token authentication is for API clients; default Web use remains loopback-only. |
| Automatic retention deletion | DEFERRED | `retention_days` is diagnostic/reserved; v0.1.0 does not silently delete audit evidence. |

## 5. New release artifacts

- `.github/workflows/ci.yml`
- `.goreleaser.yaml`
- `CHANGELOG.md`, `CONTRIBUTING.md`, `SECURITY.md`, and `LICENSE`
- `docs/release-audit.md`, `docs/release-scope-v0.1.md`,
  `docs/rbac.md`, and this report
- `scripts/demo.ps1` and `scripts/demo.sh`
- configuration/build metadata packages and additional backend/frontend tests

Because this repository currently has no commits and every file is untracked,
Git cannot provide a meaningful committed baseline for an exact added/modified
file classification.

## 6. Major modified areas

- CLI and daemon entry points under `cmd/`
- Event schema, SQLite repository, migrations, and verification under
  `internal/events/`
- Restore and content-addressed storage under `internal/snapshots/`
- Service/export, API/RBAC, capture, redaction, sync, and MCP implementations
- React/Vite Web console and embedded production assets
- README and architecture/security documentation

## 7. Database and hash compatibility

SQLite schema version 2 is applied through idempotent migrations. Existing
databases are upgraded in place with expected restore-state fields, migration
metadata, and a busy timeout. WAL remains enabled. New hashes use canonical
evidence JSON; legacy development hashes remain verifiable through a narrowly
scoped compatibility path. Database readiness uses `PRAGMA quick_check`.

## 8. CLI status

The release candidate exposes `run`, `log`, `show`, `restore`, `export`,
`access-token`, `sync`, `verify`, `doctor`, and `version`. Help output documents
restore overwrite scope and insecure sync requirements. `verify` exits nonzero
for a broken chain and identifies the first integrity failure. `doctor` checks
database/schema, snapshot storage, policy/access inputs, sync configuration,
daemon readiness, and MCP availability without printing secrets.

## 9. HTTP API status

The API supports bounded cursor pagination (maximum 500 records), explicit
integrity and rollback status, viewer read access, operator/admin rollback
requests, public non-sensitive health/readiness probes, generic 500 responses,
same-origin behavior, security headers, and request validation. The permission
matrix is documented in [rbac.md](rbac.md).

## 10. Web status

The embedded React UI loads timeline pages in bounded batches and replaces the
current page rather than rendering an unbounded event list. It renders
Verified/Broken/Unknown integrity states and
Available/Partial/Unavailable/Unknown rollback states, risk badges, diffs, and
error states. Production assets were rebuilt and embedded.

## 11. MCP status

The stdio MCP server retains `start_run`, `record_action`, `get_run_summary`, and
`request_rollback`. It uses the shared configuration and build metadata,
redacts stored evidence, and verifies rollback targets belong to the requested
run. The release binary is named `agent-audit-mcp`.

## 12. Security status

All required application-level audit findings were remediated. Local-first
storage, loopback binding, HTTPS-by-default sync, explicit RBAC, atomic restore,
tamper-evident chains, output bounds, and expanded redaction are covered by
tests. Snapshots intentionally preserve exact bytes for recovery and can
therefore contain secrets; filesystem permissions and host trust remain part of
the security boundary. This project is not an EDR and cannot observe actions
outside its CLI/MCP integration boundary.

## 13. Cross-platform status

- Windows amd64 runtime tests/builds: PASSED.
- Windows arm64 cross-build: PASSED.
- Linux amd64/arm64 cross-build: PASSED; runtime NOT VERIFIED locally.
- macOS amd64/arm64 cross-build: PASSED; runtime NOT VERIFIED locally.
- PowerShell demo: PASSED in a temporary Git repository.
- POSIX shell demo: NOT RUN on this Windows host.

## 14. Test and static-analysis results

| Gate | Result |
|---|---|
| `go test ./...` | PASSED |
| `go vet ./...` | PASSED |
| `go test -race ./...` | NOT VERIFIED locally: race requires CGO and GCC is not installed |
| `go mod verify` | PASSED |
| `npm ci --ignore-scripts` | PASSED |
| `npm test` | PASSED: 1 file, 2 tests |
| `npm run build` | PASSED |
| Six OS/architecture Go build targets | PASSED |
| Release smoke: version/help/doctor | PASSED |
| Live `auditd` health/readiness smoke | PASSED and process stopped |
| Non-loopback unauthenticated bind refusal | PASSED |

Tests cover restore conflict/force/corruption/symlink/concurrency, database
migration/restart/corruption/concurrency, hash modification/deletion/insertion/
sequence/link tampering, canonical JSON compatibility, CLI verification/doctor/
atomic export, API authentication/RBAC/pagination/readiness, redaction
persistence, sync URL/idempotency/error handling, and frontend status mapping.

## 15. Benchmark snapshot

Measured once on this Windows amd64 host (Intel Core i7-13620H):

- Write 10,000 events: 5.29 s.
- Query 10,000 events: 48.9 ms.
- Verify 10,000 events: 100.6 ms.
- Store a 1 MiB snapshot: 4.21 ms, approximately 248.84 MiB/s.

These are local development measurements, not service-level guarantees.

## 16. CI status

`.github/workflows/ci.yml` now defines Windows, Ubuntu, and macOS jobs for module
verification, tests, vet, frontend install/test/build, and binary builds. Ubuntu
also runs the CGO race detector. The workflow is present but **NOT RUN** because
the repository has no commits and no remote CI execution was authorized.

## 17. Release build status

GoReleaser configuration targets `audit`, `auditd`, and `agent-audit-mcp` for
Windows/Linux/macOS on amd64/arm64, using ZIP for Windows, tar.gz elsewhere, and
SHA-256 checksums. Direct cross-compilation of all six platform combinations
passed. A Windows release smoke build with injected v0.1.0 version, commit, and
build time passed. GoReleaser itself was not installed, so its configuration
has not been tool-validated or used to publish artifacts.

## 18. Known limitations

- Capture is limited to operations routed through supported CLI/MCP adapters.
- External side effects are not generally reversible.
- A local administrator controlling both storage and binaries is outside the
  tamper-resistance model.
- Exact snapshots may contain secrets even though event evidence and exports
  are redacted.
- Authenticated browser login and automatic retention deletion are deferred.
- The repository has no commit history, so provenance begins with the owner's
  first reviewed commit.

## 19. Release-ready decision

**NO — not yet ready for a final public v0.1.0 release.**

The implementation is Stage 4 complete and suitable as a release candidate.
The remaining final-release blockers are the owner's license decision and a
successful first hosted CI run. Race detection, Linux/macOS runtime tests, npm
vulnerability audit, and GoReleaser validation must be resolved or explicitly
accepted with evidence in that release run.

## 20. Recommendation

Do not tag or publish v0.1.0 yet. Choose and install the intended license,
review and create the first commit/PR, run the full CI workflow, run dependency
audit against a supported registry, and validate the GoReleaser configuration.
If those gates pass, create a signed/annotated v0.1.0 tag and publish the
checksummed artifacts through the normal reviewed release process.

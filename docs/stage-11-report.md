# Stage 11 report — v0.2.0 release

Release date: 2026-10-04

## Release identity

- Base branch: `feature/v0.2-mcp`
- Feature commit: `dfa695c45c0156bd1297b1fb66addf49d4119613`
- Release branch: `release/v0.2.0`
- Release hardening branch commit: `39dd8fb6d3aa2776ab525c30494bed90bd88b41c`
- Release/merge commit: `71565a7eda6f6cbfd0636ca21c5ed8b49f44c869`
- Master at release: `71565a7eda6f6cbfd0636ca21c5ed8b49f44c869`
- Annotated tag: `v0.2.0`, resolving to `71565a7eda6f6cbfd0636ca21c5ed8b49f44c869`
- Release URL: https://github.com/zzzmmmymax/agent-audit-console/releases/tag/v0.2.0

The report is committed after publication as permitted by the release plan. The
tag remains fixed at the release/merge commit and is not moved to the report
commit. The feature and release branches remain available.

## Release gates

| Gate | Result | Evidence |
| --- | --- | --- |
| Release audit | PASS | `docs/v0.2.0-release-audit.md`; all blocker/high findings resolved before tagging |
| v0.1.0 to v0.2.0 upgrade | PASS | Official v0.1.0 Windows artifact created a real run, two-event chain, diff, snapshot, and database; official v0.2.0 migrated it to schema 3 without data reset |
| CLI compatibility | PASS | v0.1 `run`, `log`, `show`, `restore`, `verify`, `doctor`, `sync`, and `access-token` remain registered and tested; v0.2 commands are additive |
| MCP compatibility | PASS | v0.1 tool names/fields remain; v2 additions are discoverable; official artifact `health`, `capabilities`, and `simulate_policy` passed with 22 tools |
| API compatibility | PASS | Existing routes/fields, RBAC, auth, and pagination remain; Stage 9/10 endpoints are additive |
| Event hash compatibility | PASS | v0.1 schema-1 canonical chain verified `VALID` after schema-3 migration; optional v0.2 metadata does not alter old payloads |
| Policy compatibility | PASS | Legacy decision aliases and team/local policies remain accepted; validation, simulation, explain, and tests passed |
| Setup compatibility | PASS | Preview default, merge preservation, unknown fields, malformed input failure, bounded backup, and atomic replacement are covered |
| Web regression | PASS | Run list/detail, timeline, action chain, diff, command, risk, approval, integrity, rollback, and Policy Inspector regression passed; missing historical policy metadata renders unavailable |
| Security audit | PASS | RBAC, approval/rollback separation, workspace/symlink containment, redaction, side-effect-free simulation, MCP ownership, idempotency, payload/YAML/RE2 bounds, XSS/control rendering, and HTTPS sync reviewed and tested |
| Secret scan | PASS | Tracked files and Git history scanned; matches are explicit synthetic redaction fixtures only; official artifacts have no credential-pattern hits |
| Privacy scan | PASS | Tracked/history/release artifacts contain no personal user path, customer data, private server, real token, or other private release data |
| Dependency gate | PASS | `go mod tidy` produced no module diff; `go mod verify`, `npm ci`, backend tests/vet, 11/11 frontend tests, and frontend production build passed |

## Regression and performance

The 10,000-event regression generated the events in 13,071.55 ms. Full hash
verification completed in 175.94 ms and returned `VALID`. API run-list/search
completed in approximately 642.33 ms; filtered detail pagination returned 100
items with `has_more=true` in approximately 636.20 ms, and the next page also
returned 100 items. Web and Policy Inspector endpoints remained responsive.

Policy benchmark results on Windows amd64 (13th Gen Intel Core i7-13620H):

| Operation | Rules | Result |
| --- | ---: | ---: |
| Evaluate | 100 | 5,565 ns/op |
| Simulate | 100 | 6,153 ns/op |
| Evaluate | 1,000 | 102,496 ns/op |
| Simulate | 1,000 | 174,418 ns/op |

The 1,000-rule simulation improved from the Stage 10 result of 218,027 ns/op;
there is no measured policy performance regression.

## Build and CI

- Snapshot dry run: PASS with GoReleaser v2.18.2; six archives and all snapshot checksums verified.
- Snapshot Windows amd64 smoke: PASS for version, doctor, policy validation/simulation, MCP, health/readiness, Web, and Policy Inspector API.
- RC CI run `37167836813`: Ubuntu PASS, Windows PASS, macOS PASS, frontend/backend/build PASS, Linux race PASS.
- Master CI run `37167986292`: Ubuntu PASS, Windows PASS, macOS PASS, frontend/backend/build PASS, Linux race PASS.
- Official GoReleaser v2.18.2: PASS (`goreleaser release --clean`).
- GitHub Release: published, not draft, not prerelease.

The GitHub Actions Node runtime warning was removed by upgrading only official
stable actions to `actions/checkout@v5`, `actions/setup-go@v6`, and
`actions/setup-node@v5`. Runner-image migration/capacity notices are
infrastructure annotations, not test failures.

## Official assets and checksums

All assets were downloaded again from the public GitHub Release rather than
validated only from local `dist`. Every archive contains `audit`, `auditd`,
`agent-audit-mcp`, `README.md`, and `LICENSE` (with `.exe` names on Windows).

| Asset | SHA-256 |
| --- | --- |
| `agent-audit-console_0.2.0_darwin_amd64.tar.gz` | `c9d4234c2716b87c88fd078926bcb0eb94f8f68711c36a89f2d258960c6729fd` |
| `agent-audit-console_0.2.0_darwin_arm64.tar.gz` | `5f032b4d5c791330c71b3686de1d2f2075d6e2efcdbf6dd25b1adb4c53c1d931` |
| `agent-audit-console_0.2.0_linux_amd64.tar.gz` | `0264d5156c0cda9f704559db548d59eadd08f379b8931296aa5389f1f51e32ee` |
| `agent-audit-console_0.2.0_linux_arm64.tar.gz` | `884d68851c5148d69755a14486914aa3d9dcfdd5d8387751bd39f4c2a403555d` |
| `agent-audit-console_0.2.0_windows_amd64.zip` | `19cd16a26def454255f326da280b8f79017ec5e1a6d37191a24785c86f2f69e7` |
| `agent-audit-console_0.2.0_windows_arm64.zip` | `079135ad536db6cecbd846d50c393bb4585ed9b82a4931663e388720460b7bb1` |

The official Windows amd64 artifact reported version `0.2.0`, commit
`71565a7eda6f6cbfd0636ca21c5ed8b49f44c869`, and schema 3. CLI doctor and
policy commands, MCP health/capabilities/simulation, HTTP health/readiness,
embedded Web, Policy Inspector, and Policy simulation all passed.

The final official upgrade test independently recreated v0.1.0 data using the
official v0.1.0 artifact. The official v0.2.0 artifact automatically migrated
the database, listed and showed the old run/events/diff, verified the old hash
chain, read the old snapshot, and restored the original `v0.1 before` content.

## Repository verification

- Repository visibility: PUBLIC
- `master`: contains the tagged release merge and this post-release report
- v0.1.0 release: retained
- v0.2.0 release: published with six platform archives and `checksums.txt`
- No force push, history rewrite, old-tag modification, or branch deletion was performed

## Known issues

- Agent Audit Console is not an OS-level EDR and captures only integrated operations.
- External effects may not be reversible.
- There is no native Codex sidebar integration.
- Full historical policy snapshot diffs are not stored; fingerprints remain available.
- Static policy shadow detection is intentionally limited to reliable identical-matcher cases.
- The Web console inspects and simulates policy but does not edit it.
- Very large diffs are truncated in the browser.
- A rollback request records intent; it does not itself prove that restoration executed.
- Go race execution is gated on Linux CI; the Windows matrix intentionally skips `-race`.

## Outcome

- Stage 11: COMPLETE
- v0.2.0 release state: RELEASED


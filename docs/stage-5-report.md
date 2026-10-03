# Stage 5 report

Report date: 2026-10-03 (Asia/Shanghai)

## Summary

| Gate | Status |
|---|---|
| Current Directory | Dedicated repository checkout (local path omitted) |
| Git Top Level | Dedicated repository checkout (local path omitted) |
| Independent Repository | YES — local `.git` directory and expected history verified |
| Current Branch | `master` |
| Current Commit | `HEAD` (the commit containing this report; resolve with `git rev-parse HEAD`) |
| GitHub Username | `zzzmmmymax` (read from `gh api user`) |
| GitHub Repository | `https://github.com/zzzmmmymax/agent-audit-console` (PRIVATE) |
| Remote | `origin` → `https://github.com/zzzmmmymax/agent-audit-console.git` |
| Initial Commit | PASS — `6f7ffa5` (`feat: initial Agent Audit Console v0.1.0 release candidate`) |
| Push Status | PASS — `master` tracks `origin/master`; no history rewrite |
| GitHub CI Status | PASS — run `37042792723` on commit `0935d10` |
| Ubuntu Runner | PASS — module verify, tests, vet, frontend, three binary builds |
| Windows Runner | PASS — module verify, tests, vet, frontend, three binary builds |
| macOS Runner | PASS — module verify, tests, vet, frontend, three binary builds |
| Race Detector | PASS — `go test -race ./...` on Ubuntu GitHub Actions |
| Go Tests | PASS — `go test ./...` |
| Go Vet | PASS — `go vet ./...` |
| Go Mod Verify | PASS — `go mod verify`; `go mod tidy` made no changes |
| Frontend Tests | PASS — 1 test file, 2 tests |
| Frontend Build | PASS — Vite production build |
| Runtime Tests | PASS on Windows for CLI version/doctor and the PowerShell demo |
| Cross-platform Tests | PASS — native Ubuntu, Windows, and macOS GitHub runners |
| Release Dry Run | PASS — GoReleaser v2.18.2 snapshot from `0935d10` |
| Artifacts | PASS — six OS/architecture archives, each containing all three binaries, README, and license notice |
| Checksum | PASS — all six archives match GoReleaser SHA-256 output |
| Artifact Smoke Test | PASS — extracted Windows amd64 archive; version, doctor, daemon, health, readiness, and embedded Web verified |
| Secret Scan | PASS — only deliberate fake credentials in redaction tests matched; artifacts contain no developer project/user path |
| Security Gate | PASS — repository, history, CI, artifacts, and local smoke test verified |
| License Status | At Stage 5 completion: TO BE DETERMINED; finalized as Apache-2.0 before v0.1.0 |
| Stage 5 | **COMPLETE** |
| Release Status | **At Stage 5 completion: RELEASE READY — WAITING FOR LICENSE DECISION** |

## Git workspace audit

The repository began Stage 5 on `master` with no commits, no remote, and all
project files untracked. The initial commit and Stage 5 status commit were then
created. The complete repository was moved to a dedicated standalone checkout; its Git
top level is that same directory, its local `.git` metadata is intact, and both
expected commits remain in the history. It is not managed by a parent or shared
repository.

The review found ignored local build binaries under `bin/` and frontend
dependencies under `web/node_modules/`; neither entered the commit. No audit
databases, runtime snapshots, logs, coverage output, local configuration,
certificates, private keys, or user data were staged.

The previous remote issue root cause is **C: the project was already an
independent repository but had never been configured with an origin**. Moving
the directory improved project isolation but did not itself create a remote.
GitHub CLI authentication and `gh api user` identified `zzzmmmymax`; a private,
empty `zzzmmmymax/agent-audit-console` repository was created and configured as
`origin` without generating remote README, license, or ignore files.

`.gitignore` was expanded to exclude local environment files, SQLite variants
and sidecars, runtime data/snapshot directories, logs, coverage, build output,
Node dependencies, certificates, private keys, and local configuration files.

## Secret scan

The working tree and the exact staged index were scanned for bearer credentials,
password/API-key assignments, GitHub/OpenAI/AWS key forms, private-key headers,
database credentials, and developer-specific absolute paths. Matches were
limited to documentation placeholders and deliberately invalid sequential-letter
fixtures used by redaction/export/sync regression tests. No real secret or
personal absolute path was found. Those fixtures were retained because removing
them would weaken the security tests.

## CI and dependency review

The workflow uses only `actions/checkout@v4`, `actions/setup-go@v5`, and
`actions/setup-node@v4`, with read-only repository contents permission. It has
no remote-script pipelines, real credentials, fixed developer paths, registry
dependencies, or test bypasses. It runs module verification, Go tests, vet,
frontend `npm ci`, frontend tests/build, and all three binary builds on Ubuntu,
Windows, and macOS. The Ubuntu job additionally runs `go test -race ./...` with
CGO enabled.

`package-lock.json` is committed and CI uses `npm ci`. `go mod tidy` changed
neither `go.mod` nor `go.sum`, and module verification passed.

The release-candidate branch was pushed without force to the private GitHub
repository. GitHub Actions run `37042792723` passed on native Ubuntu, Windows,
and macOS runners. Every runner executed module verification, Go tests, vet,
frontend install/tests/production build, and all three binary builds. Ubuntu
also executed and passed the CGO race detector.

## Local release gate

- `go test ./...`: PASS.
- `go vet ./...`: PASS.
- `go mod verify`: PASS.
- `npm ci`: PASS.
- `npm test`: PASS (2/2).
- `npm run build`: PASS.
- Direct builds of `audit`, `auditd`, and `agent-audit-mcp`: PASS.
- Windows amd64/arm64, Linux amd64/arm64, and Darwin amd64/arm64 cross-builds:
  PASS.
- `audit version` with injected release-candidate metadata: PASS.
- `audit doctor` against isolated temporary data: PASS, with expected warnings
  for optional services not configured/running.
- PowerShell demo: PASS, including command capture, file diff, chain verify,
  safe restore, and post-restore verify.
- Restore, RBAC, sync, and redaction regression suites: PASS as part of the Go
  test suite.
- Local race execution: NOT RUN. Enabling CGO fails before tests start because
  the configured C compiler `gcc` is unavailable. This no longer blocks the
  gate because the Linux GitHub runner executed and passed the race suite.

## Documentation and packaging

README now contains explicit Build, Test, and Release guidance without local
developer paths or an unselected open-source license claim. The v0.1.0 release
notes draft is in `docs/release-notes-v0.1.0.md`.

GoReleaser v2.18.2 validated the configuration and completed a snapshot from
commit `0935d10`. It produced Windows, Linux, and Darwin archives on amd64 and
arm64. Windows uses ZIP; Linux and macOS use tar.gz. Every archive contains
`audit`, `auditd`, `agent-audit-mcp`, README, and the then-current license-status
notice. All six archive hashes match `checksums.txt`.

The first dry run exposed the local Go toolchain path in binaries. This was
classified as a release-security issue and fixed by adding `-trimpath` to all
three GoReleaser builds in commit `0935d10`; CI was rerun and passed. The final
snapshot contains neither the original checkout path nor the local user path.

The Windows amd64 snapshot archive was extracted to an isolated temporary
directory. Its `audit version` reported snapshot version, commit, build time,
and Go version; `audit doctor` passed and found the packaged MCP binary. Its
`auditd` served successful `/healthz`, `/readyz`, and embedded Web Console
responses without Node.js, npm, or Vite. No release artifact was uploaded or
published.

## License

At Stage 5 completion, no license had been selected on the owner's behalf and
formal release was therefore blocked. The project owner subsequently selected
Apache License 2.0 for v0.1.0 during Stage 6; the repository now contains the
standard Apache-2.0 license text.

## Known issues and blockers

1. GitHub reports that the selected stable major versions of the official
   checkout/setup actions target deprecated Node.js 20 and are temporarily
   forced onto Node.js 24 by the runner. This warning did not fail any job and
   should be addressed when the official actions publish/adopt newer majors.
2. The configured npm mirror does not implement the vulnerability-audit API, so
   that optional query remains unverified; lockfile installation and tests pass.

## Completion decision

**Stage 5: COMPLETE**

**Release State at Stage 5 completion: RELEASE READY — WAITING FOR LICENSE DECISION**

All technical Stage 5 gates were satisfied. The license was subsequently
finalized as Apache-2.0 before the separate Stage 6 release process.

# Stage 6 completion report

Date: 2026-10-03

Stage 6 finalized the project license and published the official v0.1.0
release. The repository remains private and no v0.2.0 development was started.

## Release record

| Gate | Result |
| --- | --- |
| License | PASS — Apache License 2.0 official text |
| License Commit | `4bf0f32e4df1124029c89ca147ad29100ee3387e` |
| Release Commit | `4bf0f32e4df1124029c89ca147ad29100ee3387e` |
| Tag | PASS — annotated `v0.1.0` |
| Tag Commit | `4bf0f32e4df1124029c89ca147ad29100ee3387e` |
| GitHub Release | PASS — published, not a draft or prerelease |
| Release URL | https://github.com/zzzmmmymax/agent-audit-console/releases/tag/v0.1.0 |
| Repository Visibility | PRIVATE |
| Stage 6 | **COMPLETE** |
| Release State | **v0.1.0 RELEASED** |

The Apache-2.0 text was compared with the official Apache License 2.0 text.
README, CONTRIBUTING, CHANGELOG, release scope, release notes, and historical
stage reports were updated without adding a project-owner identity or NOTICE
file. The release archives include README and LICENSE.

## Security gate

The repository status, unstaged and staged diffs, tracked files, filenames, and
complete Git history were checked before tagging. No private key, temporary SSH
key, deployment directory, database, snapshot, user data, or real credential
was found.

Gitleaks v8.30.1 passed against both the complete five-commit history and the
working tree. A narrowly scoped rule permits only the exact synthetic AWS-key
marker in `internal/redact/redact_test.go`; that fixture exists to verify secret
redaction and is not a credential. No other path or value is allowlisted.

**Secret Scan: PASS**

## Local release gate

The following commands passed before the release commit was created:

- `go mod verify`
- `go test ./...`
- `go vet ./...`
- `npm ci` in `web`
- `npm test` in `web` — 2/2 tests passed
- `npm run build` in `web`

Windows did not run the Go race detector. The release commit's Linux GitHub
Actions job ran `go test -race ./...` successfully.

## Final GitHub CI

Workflow run: https://github.com/zzzmmmymax/agent-audit-console/actions/runs/37051065237

The run tested the exact release commit:

- Ubuntu: PASS
- Windows: PASS
- macOS: PASS
- Linux Race Detector: PASS
- Frontend test and build: PASS
- Go build for `audit`, `auditd`, and `agent-audit-mcp`: PASS

The workflow emitted non-failing platform notices about the official v4/v5
actions being forced from deprecated Node.js 20 onto Node.js 24. This did not
affect the test results.

## GoReleaser and artifacts

GoReleaser v2.18.2 validated `.goreleaser.yaml` and completed a non-snapshot
release. It injected version, release commit, build time, and Go version into
the binaries and produced:

- `agent-audit-console_0.1.0_windows_amd64.zip`
- `agent-audit-console_0.1.0_windows_arm64.zip`
- `agent-audit-console_0.1.0_linux_amd64.tar.gz`
- `agent-audit-console_0.1.0_linux_arm64.tar.gz`
- `agent-audit-console_0.1.0_darwin_amd64.tar.gz`
- `agent-audit-console_0.1.0_darwin_arm64.tar.gz`
- `checksums.txt`

All seven assets are uploaded to the published GitHub Release. All six archive
SHA-256 values were recalculated from freshly downloaded official assets and
matched `checksums.txt`.

**GoReleaser: PASS**

**Release Artifacts: PASS**

**Checksums: PASS**

## Official artifact smoke test

The Windows amd64 archive was downloaded from the published GitHub Release,
not reused from the local `dist` directory. The extracted archive contained all
three binaries, README, and the full Apache-2.0 LICENSE.

- `audit version`: PASS — version `0.1.0`, release commit, build time, and Go
  version were present. GoReleaser normalizes the leading `v` out of the
  executable version while the Git tag remains `v0.1.0`.
- `audit doctor`: PASS — configuration, data directory, schema 2 database,
  snapshots, policy, retention, and packaged MCP discovery passed. Expected
  warnings remained for intentionally unconfigured API access and remote sync.
- `auditd`: PASS — `/healthz`, `/readyz`, and the embedded Web Console returned
  HTTP 200 on a loopback-only smoke-test listener.
- `agent-audit-mcp`: PASS — the official binary started, displayed its supported
  options, exited successfully, and was discovered by `audit doctor` on PATH.

**Official Artifact Smoke Test: PASS**

## Known limitations and notices

- Agent Audit Console is not an EDR or operating-system-wide monitor.
- Operations outside supported CLI, MCP, or adapter boundaries may not be
  captured.
- External effects such as remote API operations, payments, and sent messages
  may require manual compensation and cannot always be rolled back.
- Native Codex Sidebar and Audit Card integration is not part of v0.1.0.
- Token-protected browser login and automatic retention deletion remain
  deferred.
- Exact recovery snapshots can contain sensitive source content and require
  appropriate local filesystem protection.
- The configured npm mirror does not implement the optional vulnerability-audit
  API; lockfile installation, frontend tests, and production build passed.
- GitHub Actions reported a non-failing Node.js runtime deprecation notice for
  the selected official action versions.

## Completion decision

**Stage 6: COMPLETE**

**Release State: v0.1.0 RELEASED**

The post-release report is committed after the release commit. The annotated
`v0.1.0` tag remains fixed at the tested release commit and is not moved by this
documentation update.

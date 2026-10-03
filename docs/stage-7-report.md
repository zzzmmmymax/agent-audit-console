# Stage 7 public release readiness report

Date: 2026-10-03

Stage 7 prepares the existing v0.1.0 repository for public visibility. It adds
no product feature, does not move the v0.1.0 tag, and does not republish the
release.

## Readiness summary

| Check | Result |
| --- | --- |
| README Public Readiness | PASS |
| README Quick Start | PASS |
| SECURITY.md | PASS |
| Private Vulnerability Reporting | Documented; activation to be retried immediately after public transition |
| CONTRIBUTING.md | PASS |
| Apache-2.0 LICENSE | PASS — exact official text |
| Bug Report Issue Form | PASS — YAML validated |
| Feature Request Issue Form | PASS — YAML validated |
| Issue configuration | PASS — blank issues disabled and private security link present |
| Pull Request Template | PASS |
| Secret Scan | PASS |
| Privacy Scan | PASS |
| Ubuntu CI | PASS |
| Windows CI | PASS |
| macOS CI | PASS |
| Linux Race Detector | PASS |
| Public Readiness | **READY** |

## README public readiness

README now gives a new visitor the project purpose, four audit questions,
implemented v0.1.0 features, explicit non-goals, architecture, release download
and source-build paths, first audited command, run inspection, integrity
verification, controlled restore, Web Console startup, MCP tools, Agent Adapter
scope, real YAML policy schema, RBAC, security model, limitations, development
commands, contribution guidance, security reporting, and license.

The badges and links use the real `zzzmmmymax/agent-audit-console` repository.
The release-binary path does not require Node.js. The source-development path
uses only repository-relative commands and the documented Go 1.26 and Node.js
22 prerequisites.

## Security and contribution files

SECURITY.md supports v0.1.x, distinguishes `master` from stable releases,
directs vulnerabilities away from public Issues, identifies in-scope and
out-of-scope reports without weakening real security boundaries, and documents
the coordinated disclosure process without inventing an email address or SLA.

The canonical private advisory URL is documented. GitHub's enablement endpoint
returned Not Found while the repository was private, so enablement will be
retried and verified immediately after changing visibility to public.

CONTRIBUTING.md now covers setup, builds, the full test gate, code style,
commits, pull requests, private security reporting, feature requests, and the
compatibility, migration, and security analysis required for architectural
changes.

## Community health files

The repository contains and GitHub's content API confirms:

- `README.md`
- standard Apache-2.0 `LICENSE`
- `SECURITY.md`
- `CONTRIBUTING.md`
- `.github/ISSUE_TEMPLATE/bug_report.yml`
- `.github/ISSUE_TEMPLATE/feature_request.yml`
- `.github/ISSUE_TEMPLATE/config.yml`
- `.github/pull_request_template.md`

Both Issue Forms and the configuration file passed an independent YAML syntax
check. GitHub's Community Profile API recognizes README, LICENSE, CONTRIBUTING,
and the pull request template; its aggregate issue-template field had not yet
refreshed even though the content API returned all three Issue Template files.

## Security and privacy scans

Gitleaks v8.30.1 scanned the complete Git history and the working tree with no
leaks. Its only allowlist remains restricted by both exact synthetic test value
and exact redaction-test path.

Tracked files, new public-health files, README examples, documentation, release
notes, and the worktree were checked for private keys, credential files,
databases, snapshots, customer data, and real credentials. No risky filename
was present. Current repository text contains no author-specific checkout path
or username path. Historical Stage 5 path references in the current document
were replaced with path-neutral wording.

## Test and CI evidence

The local public-readiness gate passed:

- `go mod verify`
- `go test ./...`
- `go vet ./...`
- `npm ci` in `web`
- `npm test` in `web` — 2/2 tests
- `npm run build` in `web`

GitHub Actions run:

https://github.com/zzzmmmymax/agent-audit-console/actions/runs/37099308400

The run tested public-readiness commit
`d288e7ef01d2e5e3bdd85b87907c12dff6f0064d` and passed on Ubuntu, Windows,
and macOS. The Ubuntu job also passed `go test -race ./...`.

## Release integrity

The annotated `v0.1.0` tag remains at
`4bf0f32e4df1124029c89ca147ad29100ee3387e`. The published GitHub Release still
contains the six Windows/Linux/macOS archives and `checksums.txt`. Stage 7 does
not change or republish those immutable release inputs.

## Known issues

- Current official GitHub Actions versions emit a non-failing Node.js runtime
  deprecation warning while GitHub forces them onto Node.js 24.
- The configured npm mirror does not expose the optional vulnerability-audit
  endpoint; clean installation, tests, and production build pass.
- Git smart HTTP access to `github.com:443` was intermittent on the release
  workstation. The Stage 7 commit was published through GitHub's official Git
  Data API as a verified non-force fast-forward with identical local and remote
  Git objects.
- Product limitations documented in README and the v0.1.0 release notes remain
  unchanged.

## Optional governance items

A Code of Conduct is not currently present. It is an optional future governance
decision and is not required for Stage 7 public readiness.

## Decision

**PUBLIC READINESS: READY**

All security, privacy, documentation, template, license, local-test, and final
CI gates required before the visibility transition have passed. Repository
visibility can be changed from private to public. Private vulnerability
reporting must be enabled and verified immediately after that transition.

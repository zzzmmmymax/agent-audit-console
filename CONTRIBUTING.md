# Contributing

Thank you for helping improve Agent Audit Console. Contributions should keep
the project local-first, product-neutral, cross-platform, and explicit about
security boundaries.

## Getting started

Prerequisites:

- Go 1.26 or newer;
- Node.js 22 or newer and npm for Web Console development; and
- Git.

Clone the repository and install dependencies:

```shell
git clone https://github.com/zzzmmmymax/agent-audit-console.git
cd agent-audit-console
go mod download
npm --prefix web ci
```

Build the Go binaries:

```shell
go build ./cmd/audit
go build ./cmd/auditd
go build -o agent-audit-mcp ./mcp
```

The Web Console is embedded in `auditd`. Rebuild it after frontend changes:

```shell
npm --prefix web run build
```

## Testing

Run the relevant focused tests while developing, then the complete local gate
before opening a pull request:

```shell
go mod verify
go test ./...
go vet ./...
npm --prefix web ci
npm --prefix web test
npm --prefix web run build
```

Linux contributors with a supported native toolchain should also run:

```shell
go test -race ./...
```

If a command cannot be run locally, explain why in the pull request. CI runs
the supported cross-platform matrix.

## Code style

- Format Go changes with `gofmt`.
- Follow existing package boundaries and keep public interfaces small.
- Keep frontend TypeScript type-safe and consistent with the existing build.
- Prefer clear errors that do not disclose secrets or unnecessary local paths.
- Keep comments focused on invariants, security boundaries, and non-obvious
  behavior.

New agent integrations belong behind `AgentAdapter`; policy implementations
belong behind the evaluator boundary. Do not bypass storage, redaction, policy,
containment, or integrity interfaces for convenience.

## Commit guidance

- Keep commits focused and reviewable.
- Use an imperative, descriptive subject such as `fix: reject restore escape`.
- Avoid combining unrelated refactors with behavior or security changes.
- Do not commit generated local data, credentials, audit databases, snapshots,
  temporary keys, or personal paths.

## Pull requests

- Explain the problem, motivation, and chosen approach.
- Add or update tests for behavior and security-boundary changes.
- Update README and design documents when behavior or guarantees change.
- Preserve Windows, Linux, and macOS behavior.
- Preserve migration compatibility; do not require users to delete databases.
- Do not weaken redaction, authorization, integrity, containment, or HTTPS
  checks.
- Avoid unrelated formatting or refactoring.
- Complete the pull request template and report every test command run.

## Security issues

Do not open a public issue or pull request containing an undisclosed
vulnerability. Follow the private reporting process in [SECURITY.md](SECURITY.md).

## Feature requests

Use the feature-request Issue Form. Describe the concrete problem and use case,
not only a proposed implementation. Include privacy and security implications.
Requests are evaluated against the local-first scope and do not imply a
commitment or delivery schedule.

## Architecture changes

Pull requests that change any of the following must explicitly document
backward compatibility, migration impact, and security implications:

- event schema or canonical event hashing;
- snapshot format or content-addressing behavior;
- hash-chain or trusted-head verification;
- RBAC roles or authorization semantics;
- policy evaluation or approval semantics;
- remote synchronization protocol; or
- restore planning, containment, conflict checks, or execution.

Update the relevant documents under `docs/` and include migration tests when a
stored or network format changes.

## License

Contributions are accepted under the project's [Apache License 2.0](LICENSE).

# Contributing

Thank you for helping improve Agent Audit Console.

## Development setup

Install Go 1.26 or newer and Node.js 22 or newer. Then run:

```shell
go mod download
go test ./...
go vet ./...
npm --prefix web ci
npm --prefix web test
npm --prefix web run build
```

Keep changes local-first and product-neutral. New agent integrations belong
behind `AgentAdapter`; policy implementations belong behind the evaluator
boundary. Never add real credentials, audit databases, or snapshot objects to
fixtures.

## Pull requests

- Add tests for behavior and security-boundary changes.
- Update README and design documents when commands or guarantees change.
- Preserve migration compatibility; do not require users to delete a database.
- Run the complete commands above and report anything not run.
- Keep pull requests focused. Do not combine large product features with
  release-hardening fixes.

Contributions are accepted under the project's Apache License 2.0.

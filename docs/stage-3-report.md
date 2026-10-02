# Stage 3 completion report

## Delivered

- A product-neutral `AgentAdapter` interface and registry with Codex, Claude
  Code, Cursor, and custom-agent implementations.
- Adapter selection in `audit run` and MCP run/action normalization.
- Layered shared team YAML policy via `AGENT_AUDIT_TEAM_POLICY`, with team rules
  evaluated before local rules.
- Optional bearer-token protection for the HTTP API, SHA-256-only credential
  storage, and viewer/operator/admin role hierarchy.
- `audit access-token` for generating a token and `auditd --access-file` for
  enabling API authentication.
- Explicit `audit sync` delivery of verified bundles, HTTPS enforcement,
  environment-only bearer-token input, and idempotency keys.
- Architecture and security documentation for every Stage 3 boundary.

## Verification

- Adapter tests cover all registered products and normalized event metadata.
- Team-policy tests prove shared rules take precedence over local rules.
- Authentication tests cover hashed token storage, role ordering, rejected
  anonymous API calls, accepted bearer tokens, and public health checks.
- Sync tests cover authentication headers, idempotency, chain verification, and
  plaintext HTTP refusal.
- Full Go tests, vet, binary builds, Web production build, and CLI smoke tests
  complete the stage verification.

## Retained boundaries

- Audit data remains local unless `audit sync` is explicitly invoked.
- The system audits controlled CLI/MCP operations, not every operating-system
  process.
- Permission roles are ready for future mutation endpoints; today's HTTP API is
  read-only and therefore requires only viewer access.
- Codex integration remains MCP plus local Web until an official extension API
  is available.

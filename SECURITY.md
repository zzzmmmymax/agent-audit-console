# Security Policy

## Supported versions

Security fixes currently target the latest v0.1.x release.

| Version | Supported |
| --- | --- |
| v0.1.x | Yes |
| Older or unreleased versions | No |

The `master` development branch may contain changes that have not been
released and should not be treated as equivalent to the latest stable release.

## Reporting a vulnerability

**Do not report security vulnerabilities through a public GitHub issue.**

Please use GitHub's private vulnerability reporting feature when available:

https://github.com/zzzmmmymax/agent-audit-console/security/advisories/new

If that private form is unavailable, open a minimal public issue asking the
maintainers to establish a private contact channel. Do not include the
vulnerability details, exploit, secrets, private audit bundles, databases,
snapshots, customer data, or sensitive logs in that issue.

A useful private report includes:

- affected release or commit;
- operating system and architecture;
- affected command, API route, MCP tool, or data format;
- minimal reproduction steps;
- impact and the security boundary crossed; and
- suggested mitigation, if known.

Remove tokens, private paths, and unrelated sensitive data from diagnostics.
We will make a reasonable effort to acknowledge and investigate reports
promptly, but do not promise a fixed response or remediation SLA.

## Security scope

Examples of issues that should be reported privately include:

- authentication or RBAC bypass;
- plaintext token storage or secret leakage through events, exports, Web/API
  responses, synchronization, or diagnostics;
- path traversal, symlink/junction escape, unsafe restore, or arbitrary file
  overwrite outside the documented workspace boundary;
- event hash-chain or trusted-head integrity bypass;
- remote synchronization authentication, transport, or idempotency flaws;
- command execution outside the documented CLI/MCP trust boundary;
- snapshot confidentiality or integrity failures; and
- a daemon binding or access-control behavior that contradicts the documented
  localhost-safe default.

## Out of scope

The following are normally outside this policy unless they demonstrate an
additional undocumented security-boundary failure:

- unsupported versions that are no longer maintained;
- attacks that require an already fully compromised local administrator to
  replace the database, executable, and trusted chain head together;
- the documented inability to observe operations that bypass supported CLI,
  MCP, or Adapter integrations;
- the expected inability to automatically reverse irreversible third-party
  effects such as payments or already sent messages;
- reports based only on social engineering; and
- vulnerabilities solely in unrelated third-party services.

These exclusions do not dismiss a report that shows a real additional
boundary crossing, data exposure, or unsafe behavior in Agent Audit Console.

## Disclosure process

1. The reporter submits details privately.
2. A maintainer acknowledges the report when possible.
3. The issue is reproduced, scoped, and assessed.
4. A fix and regression tests are prepared without exposing users prematurely.
5. Release and disclosure timing are coordinated with the reporter when
   appropriate.
6. A security advisory is published after a fix when public disclosure is
   appropriate.

## Security boundary

Agent Audit Console records controlled CLI, MCP, and Adapter activity. It is
not an EDR, antivirus, sandbox, or full operating-system monitor. Exact snapshot
bytes may contain secrets; protect the local data directory with operating-
system access controls. See [docs/security.md](docs/security.md) for the full
threat model and operational guidance.

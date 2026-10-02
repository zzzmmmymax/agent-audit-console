# Security policy

## Supported versions

Security fixes target the latest v0.1.x release line once v0.1.0 is published.
There is no published release at the time this file was created.

## Reporting a vulnerability

Do not include secrets, private audit bundles, databases, or snapshots in a
public issue. Use the GitHub repository's private security-advisory workflow
when available. If no private channel is configured, open a minimal issue asking
the maintainers to establish a private contact channel without disclosing the
vulnerability details.

Include the affected version/commit, operating system, reproduction boundary,
and potential impact. Remove tokens and private paths from all diagnostics.

## Security boundary

Agent Audit Console records controlled CLI and MCP activity. It is not EDR and
cannot observe operations that bypass those integration points. Snapshot bytes
are exact recovery artifacts and may contain secrets. Protect the data directory
with operating-system access controls.

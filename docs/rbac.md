# HTTP API RBAC

Bearer tokens are generated locally with `audit access-token`. The access file
stores only SHA-256 token digests. `auditd` loads the file at startup.

| Route | Viewer | Operator | Admin | Notes |
|---|---:|---:|---:|---|
| `GET /healthz` | Public | Public | Public | Liveness only; no sensitive details. |
| `GET /readyz` | Public | Public | Public | Ready/not-ready only; no paths or configuration. |
| `GET /api/whoami` | Yes | Yes | Yes | Returns token name and role. |
| `GET /api/runs` | Yes | Yes | Yes | Read-only run list. |
| `GET /api/runs/{id}` | Yes | Yes | Yes | Paginated events and integrity/rollback status. |
| `GET /api/events/{id}` | Yes | Yes | Yes | Read-only event detail. |
| `POST /api/runs/{id}/rollback-requests` | No | Yes | Yes | Creates a pending request; does not execute restore. |

There are no HTTP token-management or critical-policy mutation routes in
v0.1.0. Admin token creation remains a local CLI/filesystem operation. No
permissive CORS headers are emitted, so browser cross-origin access is denied by
default. Binding outside loopback without an access file is refused.

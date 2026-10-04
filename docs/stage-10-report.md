# Stage 10 report — Policy Simulation & Risk Explainability

- Branch: `feature/v0.2-mcp`
- Base commit: `5aa25d2`
- Final commit: Stage 10 completion commit (see repository history)
- Policy decision model: canonical `allow < warn < require_approval < deny`; legacy aliases remain accepted and persisted event statuses remain compatible.
- Risk model: independent `low < medium < high` assessment.
- Simulation: transient engine, CLI, API, MCP, and Web surfaces; no audit/approval/rollback/action side effects.
- Validation: strict YAML fields plus semantic, duplicate, matcher, size, count, overlap, and broad-rule checks.
- Testing: YAML regression runner with stable JSON/human output and built-in/default demo cases.
- Explainability: all matches, effective/overridden rules, deterministic reasons and trace.
- Fingerprint: canonical semantic SHA-256; stable across whitespace, YAML key order, and LF/CRLF.
- Historical/current: recorded metadata is distinct from explicitly re-evaluated current simulation; old metadata is unavailable, not an error.
- MCP: `simulate_policy`; richer `evaluate_action`/`explain_policy`; discovery flags for simulation, validation, trace, and tests.
- CLI: `audit policy validate|simulate|explain|test`, each with `--json`; deny simulation exits successfully.
- API/Web: viewer-safe inspection, simulation, validation and action explanation; read-only Policy Inspector.
- RBAC: viewer/operator/admin may inspect and simulate; Stage 10 exposes no policy mutation route.
- Migration: none. New policy metadata is optional inside the existing event JSON.
- Compatibility: old decision names are aliases; old events/hash chains remain valid; missing fingerprints display unavailable.
- Security: bounded documents/requests/rules/strings/RE2 patterns, strict fields, sanitized source filenames, fail-safe initial load, and last-known-good reload.

## Windows amd64 benchmark

Host CPU: 13th Gen Intel Core i7-13620H. Go benchmark, `-benchtime=300ms`:

| Operation | Rules | Time | Allocated |
| --- | ---: | ---: | ---: |
| Evaluate | 100 | 6,226 ns/op | 24,690 B/op |
| Simulate | 100 | 11,445 ns/op | 24,671 B/op |
| Evaluate | 1,000 | 171,225 ns/op | 304,735 B/op |
| Simulate | 1,000 | 218,027 ns/op | 304,851 B/op |

The 1,000-rule preflight remains below one millisecond on the measured host, so no complex index was introduced.

## Verification

- Backend tests: pass
- Frontend tests: 11/11 pass
- Frontend TypeScript/Vite build: pass
- `go vet ./...`: pass
- `go mod verify`: pass
- CI: recorded after push in the final completion response

## Known issues

- Historical full policy snapshots are intentionally not stored; fingerprint changes can be detected but a historical policy diff cannot be reconstructed.
- Static shadow detection is limited to identical matchers. Regex containment is not guessed.
- The Web surface is an inspector/simulator, not a policy editor.

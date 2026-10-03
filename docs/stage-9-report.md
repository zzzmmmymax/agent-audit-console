# Stage 9 Report — Web Audit Experience v2

- **Branch:** `feature/v0.2-mcp`
- **Base Commit:** `db554be`
- **Final Commit:** Stage 9 branch HEAD recorded in the final handoff (this report
  is included in that commit).
- **Run List v2:** PASS — aggregate metadata, risk/integrity/rollback status,
  cursor pagination, search, and server filters.
- **Run Detail v2:** PASS — full run header and compact investigation summary.
- **Session Timeline:** PASS — 100-event server pages grouped by action.
- **Action Grouping:** PASS — exact `action_id` grouping without inference.
- **Parent / Child:** PASS — recorded hierarchy, collapse/expand, child counts.
- **Diff:** PASS — unified/split renderer, line numbers, bounded rendering.
- **Commands:** PASS — collapsed output, copy, CWD/exit/duration when recorded,
  explicit test evidence, ANSI/control normalization.
- **Risk:** PASS — level, decision, matched rule, reason, unavailable source.
- **Approval:** PASS — states and recorded requester/resolver evidence.
- **Integrity:** PASS — real backend verification and first broken event.
- **Rollback:** PASS — current/target hashes, conflicts, warnings, role-aware
  request controls, detailed confirmation, broken-integrity guard.
- **Search:** PASS — server-side run and timeline search.
- **Filters:** PASS — run agent/status/risk/integrity/repository/date and timeline
  kind/risk/status.
- **Deep Links:** PASS — run/action/event URL state and local copy link.
- **RBAC:** PASS — viewer read access; operator/admin request access; backend
  authorization remains authoritative.
- **Performance:** PASS — tested with 100, 1,000, and 10,000 synthetic events;
  100-event browser pages prevent unbounded DOM growth.
- **Security:** PASS — React text escaping, no `dangerouslySetInnerHTML`, bounded
  logs/diffs, ANSI/control normalization, CSP, parameterized SQL, workspace
  containment, no snapshot object-path disclosure.
- **Frontend Tests:** PASS — model behavior, hierarchy, filters/deep links,
  log safety, RBAC, and large-list bound.
- **Backend Tests:** PASS — aggregation, explicit tests, event filters/search,
  cursor pagination, endpoints, verification, and RBAC regression.
- **CI:** PASS on implementation commit `c543e51` in run `37134159756` — Ubuntu,
  Windows, macOS, frontend test/build, and the Linux race detector are green.
- **Known Issues:** old records can display fields as unavailable when the
  evidence did not record them; integrity filtering verifies candidate runs and
  is intentionally more expensive; diff output is truncated in-browser rather
  than fetched in chunks; rollback execution is outside this UI.

Stage 9 is complete. `v0.2.0` remains in development; no merge, tag, or release
is part of this stage.


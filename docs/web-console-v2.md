# Web Audit Experience v2

Web Audit Experience v2 turns the embedded local console into an investigation
surface for one agent run. It remains local-first and renders only persisted,
redacted audit evidence. Agent, command, log, diff, and JSON values are treated
as untrusted text.

## Run list

The run list uses `GET /api/run-overviews` and server-side search, filters, and
cursor pagination. Each row exposes agent, status, repository, branch, start
time, duration, action/file/command/approval counts, risk counts, and integrity.
Filters cover agent, status, risk, integrity, repository, and date range. Search
covers run ID, repository, agent, and branch.

## Run detail and session timeline

The header shows agent/version, repository, branch, commits, status, start/end,
duration, integrity, rollback availability, and pending approvals. The compact
summary reports actions, events, file changes, commands, explicit test evidence,
approvals, and high-risk events.

The timeline groups persisted events by `action_id` and nests them only when a
recorded `parent_action_id` matches. No relationship is inferred. An action row
shows lifecycle status, risk, policy decision, children, commands, file changes,
tests, and approvals. Expanding it reveals its event evidence. The hierarchy and
selected evidence panel form the action-chain view: parent/current/children and
related operations remain visible together.

Timeline kind, risk, status, and text filters execute on the server. A page is
limited to 100 events, so a 10,000-event run does not produce 10,000 DOM nodes.
Running runs refresh every three seconds; polling stops for terminal runs.

## Evidence, commands, and diffs

Command output is collapsed by default, ANSI/control characters are normalized,
and browser rendering is capped. Commands show CWD, exit code, duration, time,
action, risk, and an explicit PASS/FAIL only when evidence identifies a test.
Redaction markers are labelled instead of being presented as accidental loss.

File evidence shows path, recorded change type, before/after hashes, action,
risk, reversibility, and source. Missing source is displayed as `Unknown source`.
The existing diff renderer supports unified and side-by-side views with line
numbers. Browser rendering is capped at 120 KB and reports truncation; raw
persisted evidence is not modified.

## Investigation panels

- **Risk:** low/medium/high items, decision, matched rule, reason, and an explicit
  unavailable label when policy source was not recorded.
- **Approval:** pending/approved/rejected evidence and available requester,
  reason, timestamps, and resolver fields. Viewer access stays read-only.
- **Integrity:** `POST /api/runs/{id}/verify` performs a real hash-chain check.
  Broken runs show the first sequence/event returned by verification and disable
  rollback requests in the UI.
- **Rollback:** `GET /api/runs/{id}/rollback-preview` hashes current workspace
  files and compares recorded expected state without exposing snapshot object
  paths. Conflicts, hashes, and warnings are visible before a pending request is
  created. The backend still requires operator/admin authorization.

Raw event JSON is collapsed and React escapes it as text. No HTML injection API
is used.

## URLs and deep links

The query string preserves `run`, `action`, and `event`; run search is also
preserved. Copy-link controls always copy the current local URL. Refreshing a
deep link restores the selected investigation target when it exists on the
current page.

## Demo data

PowerShell:

```powershell
./scripts/generate-demo-audit.ps1 -DataDir .tmp/web-audit-demo -LargeEvents 10000
go run ./cmd/auditd --data-dir .tmp/web-audit-demo
```

Linux/macOS:

```sh
./scripts/generate-demo-audit.sh .tmp/web-audit-demo 10000
go run ./cmd/auditd --data-dir .tmp/web-audit-demo
```

The generator creates only synthetic completed, failed, high-risk,
pending-approval, rollback-conflict, broken-integrity, and large-timeline runs.

## Performance observations

On the Stage 9 Windows development host, synthetic generation completed in
approximately 3.9 s / 1.8 s / 6.8 s for 100 / 1,000 / 10,000 large-run events.
Against the 10,000-event database, the run-list request was approximately 173
ms, a 100-event detail page approximately 173 ms, and a selective timeline
search approximately 209 ms. These are observations, not stable benchmarks.
The browser inspection confirmed that only 100 events were mounted per page.

## Compatibility and limitations

Existing `/api/runs`, `/api/runs/{id}`, and `/api/events/{id}` consumers remain
compatible; new fields and endpoints are additive. Evidence that was never
recorded cannot be reconstructed: policy source, command duration/CWD, hashes,
approval actors, or a moved-file classification can therefore appear as
`not recorded`/`Unknown source`. Rollback remains a request workflow; this UI
does not execute restoration.


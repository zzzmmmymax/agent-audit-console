PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
PRAGMA synchronous = FULL;
PRAGMA busy_timeout = 5000;

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS runs (
    run_id TEXT PRIMARY KEY,
    agent_type TEXT NOT NULL,
    agent_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('created', 'running', 'completed', 'failed', 'cancelled')),
    workspace_path TEXT NOT NULL,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    next_sequence INTEGER NOT NULL DEFAULT 1 CHECK (next_sequence >= 1),
    head_hash TEXT NOT NULL DEFAULT '' CHECK (head_hash = '' OR length(head_hash) = 64),
    metadata_json TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(metadata_json))
) STRICT;

CREATE TABLE IF NOT EXISTS events (
    event_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(run_id) ON DELETE RESTRICT,
    action_id TEXT NOT NULL,
    parent_action_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    action_status TEXT NOT NULL DEFAULT '' CHECK (action_status IN ('', 'planned', 'started', 'completed', 'failed', 'blocked', 'cancelled')),
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version IN (1, 2)),
    timestamp TEXT NOT NULL,
    sequence INTEGER NOT NULL CHECK (sequence >= 1),
    actor_json TEXT NOT NULL CHECK (json_valid(actor_json)),
    kind TEXT NOT NULL CHECK (kind IN ('command', 'file_change', 'git', 'mcp_call', 'approval', 'rollback')),
    intent TEXT NOT NULL,
    evidence_json TEXT NOT NULL CHECK (json_valid(evidence_json)),
    risk_json TEXT NOT NULL CHECK (json_valid(risk_json)),
    policy_decision_json TEXT NOT NULL CHECK (json_valid(policy_decision_json)),
    reversibility_json TEXT NOT NULL CHECK (json_valid(reversibility_json)),
    previous_hash TEXT NOT NULL CHECK (previous_hash = '' OR length(previous_hash) = 64),
    integrity_hash TEXT NOT NULL CHECK (length(integrity_hash) = 64),
    created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    UNIQUE (run_id, sequence),
    UNIQUE (run_id, integrity_hash)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_events_run_timestamp ON events(run_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_events_action ON events(action_id);
CREATE INDEX IF NOT EXISTS idx_events_correlation ON events(correlation_id);
CREATE INDEX IF NOT EXISTS idx_events_kind ON events(kind);
CREATE INDEX IF NOT EXISTS idx_events_risk_level ON events(json_extract(risk_json, '$.level'));
CREATE INDEX IF NOT EXISTS idx_events_policy_status ON events(json_extract(policy_decision_json, '$.status'));

CREATE TABLE IF NOT EXISTS snapshots (
    snapshot_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(run_id) ON DELETE RESTRICT,
    action_id TEXT NOT NULL,
    file_path TEXT NOT NULL,
    exists_before INTEGER NOT NULL CHECK (exists_before IN (0, 1)),
    content_hash TEXT CHECK (content_hash IS NULL OR length(content_hash) = 64),
    object_path TEXT,
    byte_size INTEGER NOT NULL CHECK (byte_size >= 0),
    file_mode INTEGER,
    captured_at TEXT NOT NULL,
    verified_at TEXT,
    expected_exists INTEGER CHECK (expected_exists IS NULL OR expected_exists IN (0, 1)),
    expected_hash TEXT CHECK (expected_hash IS NULL OR length(expected_hash) = 64),
    CHECK ((exists_before = 0 AND content_hash IS NULL AND object_path IS NULL) OR
           (exists_before = 1 AND content_hash IS NOT NULL AND object_path IS NOT NULL)),
    UNIQUE (run_id, action_id, file_path)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_snapshots_run_file ON snapshots(run_id, file_path);
CREATE INDEX IF NOT EXISTS idx_snapshots_content_hash ON snapshots(content_hash);

CREATE TABLE IF NOT EXISTS rollback_records (
    rollback_id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(run_id) ON DELETE RESTRICT,
    requested_by TEXT NOT NULL,
    target_event_id TEXT REFERENCES events(event_id) ON DELETE RESTRICT,
    target_snapshot_id TEXT REFERENCES snapshots(snapshot_id) ON DELETE RESTRICT,
    status TEXT NOT NULL CHECK (status IN ('planned', 'pending', 'executing', 'completed', 'failed', 'cancelled')),
    plan_json TEXT NOT NULL CHECK (json_valid(plan_json)),
    result_json TEXT CHECK (result_json IS NULL OR json_valid(result_json)),
    requested_at TEXT NOT NULL,
    completed_at TEXT,
    CHECK (target_event_id IS NOT NULL OR target_snapshot_id IS NOT NULL)
) STRICT;

CREATE INDEX IF NOT EXISTS idx_rollbacks_run_requested ON rollback_records(run_id, requested_at);

CREATE TABLE IF NOT EXISTS action_records (
    run_id TEXT NOT NULL REFERENCES runs(run_id) ON DELETE RESTRICT,
    action_id TEXT NOT NULL,
    parent_action_id TEXT NOT NULL DEFAULT '',
    correlation_id TEXT NOT NULL DEFAULT '',
    kind TEXT NOT NULL,
    intent TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('planned', 'started', 'completed', 'failed', 'blocked', 'cancelled')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (run_id, action_id)
) STRICT;

CREATE TABLE IF NOT EXISTS run_contexts (
    context_key TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(run_id) ON DELETE RESTRICT,
    updated_at TEXT NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS idempotency_keys (
    scope TEXT NOT NULL,
    key TEXT NOT NULL,
    response_json TEXT NOT NULL CHECK (json_valid(response_json)),
    created_at TEXT NOT NULL,
    PRIMARY KEY (scope, key)
) STRICT;

INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
INSERT OR IGNORE INTO schema_migrations(version, applied_at)
VALUES (3, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));

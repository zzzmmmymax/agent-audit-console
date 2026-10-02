package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("not found")

type SQLiteStore struct{ db *sql.DB }

func OpenSQLite(path string) (*SQLiteStore, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(SQLiteSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize schema: %w", err)
	}
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	columns, err := tableColumns(db, "snapshots")
	if err != nil {
		return err
	}
	if !columns["expected_exists"] {
		if _, err := db.Exec(`ALTER TABLE snapshots ADD COLUMN expected_exists INTEGER CHECK (expected_exists IS NULL OR expected_exists IN (0, 1))`); err != nil {
			return err
		}
	}
	if !columns["expected_hash"] {
		if _, err := db.Exec(`ALTER TABLE snapshots ADD COLUMN expected_hash TEXT CHECK (expected_hash IS NULL OR length(expected_hash) = 64)`); err != nil {
			return err
		}
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL) STRICT;
		INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (1, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));
		INSERT OR IGNORE INTO schema_migrations(version, applied_at) VALUES (2, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))`)
	return err
}

func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, kind string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(&cid, &name, &kind, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		result[name] = true
	}
	return result, rows.Err()
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) Ready(ctx context.Context) error {
	var result string
	if err := s.db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite quick_check: %s", result)
	}
	return nil
}

func (s *SQLiteStore) SchemaVersion(ctx context.Context) (int, error) {
	var version int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version),0) FROM schema_migrations`).Scan(&version)
	return version, err
}

func (s *SQLiteStore) CreateRun(ctx context.Context, run Run) error {
	metadata, err := json.Marshal(run.Metadata)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO runs
		(run_id,agent_type,agent_id,status,workspace_path,started_at,metadata_json) VALUES(?,?,?,?,?,?,?)`,
		run.RunID, run.AgentType, run.AgentID, run.Status, run.WorkspacePath, timeText(run.StartedAt), string(metadata))
	return err
}

func (s *SQLiteStore) FinishRun(ctx context.Context, id, status string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE runs SET status=?, ended_at=? WHERE run_id=?`, status, timeText(at), id)
	if err != nil {
		return err
	}
	return changed(result)
}

func (s *SQLiteStore) GetRun(ctx context.Context, id string) (Run, error) {
	return scanRun(s.db.QueryRowContext(ctx, runSelect+` WHERE run_id=?`, id))
}

func (s *SQLiteStore) ListRuns(ctx context.Context, limit int) ([]Run, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, runSelect+` ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Run
	for rows.Next() {
		item, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

const runSelect = `SELECT run_id,agent_type,agent_id,status,workspace_path,started_at,ended_at,head_hash,metadata_json FROM runs`

type scanner interface{ Scan(...any) error }

func scanRun(row scanner) (Run, error) {
	var item Run
	var started, metadata string
	var ended sql.NullString
	if err := row.Scan(&item.RunID, &item.AgentType, &item.AgentID, &item.Status, &item.WorkspacePath, &started, &ended, &item.HeadHash, &metadata); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}
	var err error
	item.StartedAt, err = parseTime(started)
	if err != nil {
		return item, err
	}
	if ended.Valid {
		value, err := parseTime(ended.String)
		if err != nil {
			return item, err
		}
		item.EndedAt = &value
	}
	if err := json.Unmarshal([]byte(metadata), &item.Metadata); err != nil {
		return item, err
	}
	return item, nil
}

func (s *SQLiteStore) Append(ctx context.Context, event Event) (Event, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()
	var sequence uint64
	var previous string
	if err := tx.QueryRowContext(ctx, `SELECT next_sequence,head_hash FROM runs WHERE run_id=?`, event.RunID).Scan(&sequence, &previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Event{}, ErrNotFound
		}
		return Event{}, err
	}
	event.Sequence = sequence
	event, err = Seal(event, previous)
	if err != nil {
		return Event{}, err
	}
	actor, err := json.Marshal(event.Actor)
	if err != nil {
		return Event{}, err
	}
	evidence, err := json.Marshal(event.Evidence)
	if err != nil {
		return Event{}, err
	}
	risk, err := json.Marshal(event.Risk)
	if err != nil {
		return Event{}, err
	}
	decision, err := json.Marshal(event.PolicyDecision)
	if err != nil {
		return Event{}, err
	}
	reversibility, err := json.Marshal(event.Reversibility)
	if err != nil {
		return Event{}, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO events(event_id,run_id,action_id,timestamp,sequence,actor_json,kind,intent,evidence_json,risk_json,policy_decision_json,reversibility_json,previous_hash,integrity_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		event.EventID, event.RunID, event.ActionID, timeText(event.Timestamp), event.Sequence, string(actor), event.Kind, event.Intent, string(evidence), string(risk), string(decision), string(reversibility), event.PreviousHash, event.IntegrityHash)
	if err != nil {
		return Event{}, err
	}
	result, err := tx.ExecContext(ctx, `UPDATE runs SET next_sequence=?,head_hash=? WHERE run_id=? AND next_sequence=?`, sequence+1, event.IntegrityHash, event.RunID, sequence)
	if err != nil {
		return Event{}, err
	}
	if err := changed(result); err != nil {
		return Event{}, errors.New("concurrent append conflict")
	}
	if err := tx.Commit(); err != nil {
		return Event{}, err
	}
	return event, nil
}

const eventSelect = `SELECT event_id,run_id,action_id,timestamp,sequence,actor_json,kind,intent,evidence_json,risk_json,policy_decision_json,reversibility_json,previous_hash,integrity_hash FROM events`

func (s *SQLiteStore) ByID(ctx context.Context, id string) (Event, error) {
	return scanEvent(s.db.QueryRowContext(ctx, eventSelect+` WHERE event_id=?`, id))
}

func (s *SQLiteStore) Query(ctx context.Context, query Query) ([]Event, error) {
	return queryEvents(ctx, s.db, query)
}

type rowQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func queryEvents(ctx context.Context, source rowQueryer, query Query) ([]Event, error) {
	if query.Limit <= 0 || query.Limit > 10000 {
		query.Limit = 1000
	}
	statement := eventSelect + ` WHERE run_id=? AND sequence>?`
	args := []any{query.RunID, query.After}
	if len(query.Kinds) > 0 {
		marks := make([]string, len(query.Kinds))
		for i, kind := range query.Kinds {
			marks[i] = "?"
			args = append(args, kind)
		}
		statement += ` AND kind IN (` + strings.Join(marks, ",") + `)`
	}
	statement += ` ORDER BY sequence LIMIT ?`
	args = append(args, query.Limit)
	rows, err := source.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Event
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

type IntegrityError struct {
	Sequence uint64
	EventID  string
	Reason   string
}

func (e *IntegrityError) Error() string {
	if e.EventID == "" {
		return fmt.Sprintf("integrity failure at sequence %d: %s", e.Sequence, e.Reason)
	}
	return fmt.Sprintf("integrity failure at sequence %d (%s): %s", e.Sequence, e.EventID, e.Reason)
}

func (s *SQLiteStore) VerifyRun(ctx context.Context, id string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	run, err := scanRun(tx.QueryRowContext(ctx, runSelect+` WHERE run_id=?`, id))
	if err != nil {
		return err
	}
	var previous string
	var after, count uint64
	for {
		items, err := queryEvents(ctx, tx, Query{RunID: id, After: after, Limit: 1000})
		if err != nil {
			return err
		}
		if len(items) == 0 {
			break
		}
		for _, event := range items {
			count++
			if event.Sequence != count {
				return &IntegrityError{Sequence: event.Sequence, EventID: event.EventID, Reason: "non-contiguous sequence"}
			}
			if event.PreviousHash != previous {
				return &IntegrityError{Sequence: event.Sequence, EventID: event.EventID, Reason: "previous hash mismatch"}
			}
			matches, hashErr := hashMatches(event)
			if hashErr != nil {
				return &IntegrityError{Sequence: event.Sequence, EventID: event.EventID, Reason: hashErr.Error()}
			}
			if !validDigest(event.IntegrityHash) || !matches {
				return &IntegrityError{Sequence: event.Sequence, EventID: event.EventID, Reason: ErrIntegrityMismatch.Error()}
			}
			previous, after = event.IntegrityHash, event.Sequence
		}
		if len(items) < 1000 {
			break
		}
	}
	var next uint64
	var head string
	if err := tx.QueryRowContext(ctx, `SELECT next_sequence,head_hash FROM runs WHERE run_id=?`, id).Scan(&next, &head); err != nil {
		return err
	}
	if next != count+1 {
		return &IntegrityError{Sequence: count + 1, Reason: "run next_sequence does not match event count"}
	}
	if head != previous || head != run.HeadHash {
		return &IntegrityError{Sequence: count, Reason: "run head hash does not match event tail"}
	}
	return tx.Commit()
}

func scanEvent(row scanner) (Event, error) {
	var item Event
	var timestamp, actor, evidence, risk, decision, reversibility, kind string
	if err := row.Scan(&item.EventID, &item.RunID, &item.ActionID, &timestamp, &item.Sequence, &actor, &kind, &item.Intent, &evidence, &risk, &decision, &reversibility, &item.PreviousHash, &item.IntegrityHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}
	var err error
	item.Timestamp, err = parseTime(timestamp)
	if err != nil {
		return item, err
	}
	item.Kind = Kind(kind)
	for _, pair := range []struct {
		raw    string
		target any
	}{{actor, &item.Actor}, {evidence, &item.Evidence}, {risk, &item.Risk}, {decision, &item.PolicyDecision}, {reversibility, &item.Reversibility}} {
		if err := json.Unmarshal([]byte(pair.raw), pair.target); err != nil {
			return item, err
		}
	}
	return item, nil
}

func (s *SQLiteStore) RecordSnapshot(ctx context.Context, item SnapshotRecord) error {
	var hash, object any
	if item.Exists {
		hash = item.ContentHash
		object = item.ObjectPath
	}
	var expectedExists, expectedHash any
	if item.ExpectedExists != nil {
		expectedExists = *item.ExpectedExists
		if *item.ExpectedExists {
			expectedHash = item.ExpectedHash
		}
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO snapshots(snapshot_id,run_id,action_id,file_path,exists_before,content_hash,object_path,byte_size,file_mode,captured_at,expected_exists,expected_hash) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`, item.SnapshotID, item.RunID, item.ActionID, item.FilePath, item.Exists, hash, object, item.ByteSize, item.FileMode, timeText(item.CapturedAt), expectedExists, expectedHash)
	return err
}

const snapshotSelect = `SELECT snapshot_id,run_id,action_id,file_path,exists_before,content_hash,object_path,byte_size,file_mode,captured_at,verified_at,expected_exists,expected_hash FROM snapshots`

func (s *SQLiteStore) LatestSnapshot(ctx context.Context, path string) (SnapshotRecord, error) {
	return scanSnapshot(s.db.QueryRowContext(ctx, snapshotSelect+` WHERE file_path=? ORDER BY captured_at DESC LIMIT 1`, path))
}
func (s *SQLiteStore) SnapshotByID(ctx context.Context, id string) (SnapshotRecord, error) {
	return scanSnapshot(s.db.QueryRowContext(ctx, snapshotSelect+` WHERE snapshot_id=?`, id))
}

func (s *SQLiteStore) RollbackAvailability(ctx context.Context, runID string) (string, error) {
	var snapshots int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM snapshots WHERE run_id=?`, runID).Scan(&snapshots); err != nil {
		return "unknown", err
	}
	if snapshots > 0 {
		return "available", nil
	}
	var partial int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE run_id=? AND json_extract(reversibility_json,'$.status') IN ('reversible','partially_reversible')`, runID).Scan(&partial); err != nil {
		return "unknown", err
	}
	if partial > 0 {
		return "partial", nil
	}
	return "unavailable", nil
}
func scanSnapshot(row scanner) (SnapshotRecord, error) {
	var item SnapshotRecord
	var hash, object, verified, expectedHash sql.NullString
	var expectedExists sql.NullBool
	var captured string
	if err := row.Scan(&item.SnapshotID, &item.RunID, &item.ActionID, &item.FilePath, &item.Exists, &hash, &object, &item.ByteSize, &item.FileMode, &captured, &verified, &expectedExists, &expectedHash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return item, ErrNotFound
		}
		return item, err
	}
	item.ContentHash = hash.String
	item.ObjectPath = object.String
	item.ExpectedHash = expectedHash.String
	if expectedExists.Valid {
		value := expectedExists.Bool
		item.ExpectedExists = &value
	}
	var err error
	item.CapturedAt, err = parseTime(captured)
	if err != nil {
		return item, err
	}
	if verified.Valid {
		value, e := parseTime(verified.String)
		if e != nil {
			return item, e
		}
		item.VerifiedAt = &value
	}
	return item, nil
}

func (s *SQLiteStore) CreateRollback(ctx context.Context, item RollbackRecord) error {
	var eventID, snapshotID, result, completed any
	if item.TargetEventID != "" {
		eventID = item.TargetEventID
	}
	if item.TargetSnapshotID != "" {
		snapshotID = item.TargetSnapshotID
	}
	if len(item.Result) > 0 {
		result = string(item.Result)
	}
	if item.CompletedAt != nil {
		completed = timeText(*item.CompletedAt)
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO rollback_records(rollback_id,run_id,requested_by,target_event_id,target_snapshot_id,status,plan_json,result_json,requested_at,completed_at) VALUES(?,?,?,?,?,?,?,?,?,?)`, item.RollbackID, item.RunID, item.RequestedBy, eventID, snapshotID, item.Status, string(item.Plan), result, timeText(item.RequestedAt), completed)
	return err
}

func (s *SQLiteStore) CompleteRollback(ctx context.Context, id, status string, result json.RawMessage, completed time.Time) error {
	changedResult, err := s.db.ExecContext(ctx, `UPDATE rollback_records SET status=?,result_json=?,completed_at=? WHERE rollback_id=?`, status, string(result), timeText(completed), id)
	if err != nil {
		return err
	}
	return changed(changedResult)
}

func timeText(value time.Time) string           { return value.UTC().Format(time.RFC3339Nano) }
func parseTime(value string) (time.Time, error) { return time.Parse(time.RFC3339Nano, value) }
func changed(result sql.Result) error {
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrNotFound
	}
	return nil
}

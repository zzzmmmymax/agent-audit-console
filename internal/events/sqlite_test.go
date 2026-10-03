package events

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLiteStoreAppendsAndVerifiesRun(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	run := Run{RunID: "run-test", AgentType: "test", AgentID: "test-agent", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		event := testEvent(1)
		event.EventID = NewID("event")
		event.RunID = run.RunID
		event.Sequence = 0
		event.Evidence.Data = json.RawMessage(`{"ok":true}`)
		sealed, err := store.Append(ctx, event)
		if err != nil {
			t.Fatal(err)
		}
		if sealed.Sequence != uint64(i+1) {
			t.Fatalf("sequence=%d", sealed.Sequence)
		}
	}
	if err := store.VerifyRun(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
	items, err := store.Query(ctx, Query{RunID: run.RunID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("events=%d", len(items))
	}
}

func TestSQLiteRestartAndMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	legacy := strings.ReplaceAll(SQLiteSchema, "    expected_exists INTEGER CHECK (expected_exists IS NULL OR expected_exists IN (0, 1)),\n", "")
	legacy = strings.ReplaceAll(legacy, "    expected_hash TEXT CHECK (expected_hash IS NULL OR length(expected_hash) = 64),\n", "")
	for _, line := range []string{"    parent_action_id TEXT NOT NULL DEFAULT '',\n", "    correlation_id TEXT NOT NULL DEFAULT '',\n", "    action_status TEXT NOT NULL DEFAULT '' CHECK (action_status IN ('', 'planned', 'started', 'completed', 'failed', 'blocked', 'cancelled')),\n", "    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version IN (1, 2)),\n", "CREATE INDEX IF NOT EXISTS idx_events_correlation ON events(correlation_id);\n"} {
		legacy = strings.ReplaceAll(legacy, line, "")
	}
	if start := strings.Index(legacy, "CREATE TABLE IF NOT EXISTS action_records"); start >= 0 {
		end := strings.Index(legacy[start:], "INSERT OR IGNORE INTO schema_migrations(version, applied_at)\nVALUES (1")
		if end < 0 {
			t.Fatal("legacy schema marker missing")
		}
		legacy = legacy[:start] + legacy[start+end:]
	}
	legacy = strings.ReplaceAll(legacy, "INSERT OR IGNORE INTO schema_migrations(version, applied_at)\nVALUES (3, strftime('%Y-%m-%dT%H:%M:%fZ', 'now'));\n", "")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(legacy); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		store, err := OpenSQLite(path)
		if err != nil {
			t.Fatal(err)
		}
		version, err := store.SchemaVersion(context.Background())
		if err != nil || version != 3 {
			t.Fatalf("version=%d err=%v", version, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestV1AndV2HashCompatibility(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	run := Run{RunID: "run-hash-versions", AgentType: "custom", AgentID: "test", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC()}
	if err = store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	v1 := testEvent(1)
	v1.EventID = "event-v1"
	v1.RunID = run.RunID
	v1.Sequence = 0
	v1.SchemaVersion = 1
	if _, err = store.Append(ctx, v1); err != nil {
		t.Fatal(err)
	}
	v2 := testEvent(1)
	v2.EventID = "event-v2"
	v2.RunID = run.RunID
	v2.Sequence = 0
	v2.SchemaVersion = 2
	v2.ParentActionID = "parent"
	v2.CorrelationID = "correlation"
	v2.ActionStatus = ActionCompleted
	if _, err = store.Append(ctx, v2); err != nil {
		t.Fatal(err)
	}
	if err = store.VerifyRun(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteRejectsCorruptDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.db")
	if err := os.WriteFile(path, []byte("not a sqlite database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if store, err := OpenSQLite(path); err == nil {
		store.Close()
		t.Fatal("corrupt database accepted")
	}
}

func TestSQLiteConcurrentAppendAndRead(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	run := Run{RunID: "run-concurrent", AgentType: "test", AgentID: "test", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errorsSeen := make(chan error, 40)
	for i := 0; i < 20; i++ {
		group.Add(2)
		go func() {
			defer group.Done()
			event := testEvent(1)
			event.EventID, event.RunID, event.Sequence = NewID("event"), run.RunID, 0
			_, err := store.Append(ctx, event)
			errorsSeen <- err
		}()
		go func() {
			defer group.Done()
			_, err := store.ListRuns(ctx, 10)
			errorsSeen <- err
		}()
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := store.VerifyRun(ctx, run.RunID); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyRunDetectsDatabaseTampering(t *testing.T) {
	for _, test := range []struct {
		name   string
		tamper func(*SQLiteStore) error
	}{{"modify", func(store *SQLiteStore) error {
		_, err := store.db.Exec(`UPDATE events SET intent='tampered' WHERE sequence=1`)
		return err
	}}, {"delete", func(store *SQLiteStore) error {
		_, err := store.db.Exec(`DELETE FROM events WHERE sequence=2`)
		return err
	}}, {"insert", func(store *SQLiteStore) error {
		fake := strings.Repeat("b", 64)
		if _, err := store.db.Exec(`INSERT INTO events(event_id,run_id,action_id,timestamp,sequence,actor_json,kind,intent,evidence_json,risk_json,policy_decision_json,reversibility_json,previous_hash,integrity_hash) SELECT 'event-injected',run_id,action_id,timestamp,3,actor_json,kind,intent,evidence_json,risk_json,policy_decision_json,reversibility_json,integrity_hash,? FROM events WHERE sequence=2`, fake); err != nil {
			return err
		}
		_, err := store.db.Exec(`UPDATE runs SET next_sequence=4,head_hash=?`, fake)
		return err
	}}, {"sequence", func(store *SQLiteStore) error {
		_, err := store.db.Exec(`UPDATE events SET sequence=3 WHERE sequence=2`)
		return err
	}}, {"previous hash", func(store *SQLiteStore) error {
		_, err := store.db.Exec(`UPDATE events SET previous_hash=? WHERE sequence=2`, strings.Repeat("a", 64))
		return err
	}}} {
		t.Run(test.name, func(t *testing.T) {
			store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			run := Run{RunID: "run-tamper", AgentType: "test", AgentID: "test", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
			if err := store.CreateRun(context.Background(), run); err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				event := testEvent(1)
				event.EventID, event.RunID, event.Sequence = NewID("event"), run.RunID, 0
				if _, err := store.Append(context.Background(), event); err != nil {
					t.Fatal(err)
				}
			}
			if err := test.tamper(store); err != nil {
				t.Fatal(err)
			}
			if err := store.VerifyRun(context.Background(), run.RunID); err == nil {
				t.Fatal("tampering was not detected")
			}
		})
	}
}

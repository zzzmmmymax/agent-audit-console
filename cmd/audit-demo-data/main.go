// Command audit-demo-data creates deterministic-shape, synthetic audit records
// for exercising the Web Audit Experience. It never reads a real repository.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
	_ "modernc.org/sqlite"
)

func main() {
	dataDir := flag.String("data-dir", filepath.Join(".tmp", "web-audit-demo"), "isolated output directory")
	large := flag.Int("large-events", 1000, "event count for the large timeline run")
	flag.Parse()
	abs, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(abs, 0o700); err != nil {
		log.Fatal(err)
	}
	dbPath := filepath.Join(abs, "audit.db")
	store, err := events.OpenSQLite(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	now := time.Now().UTC().Add(-15 * time.Minute)
	types := []struct {
		name, status string
		risk         string
		approval     bool
	}{{"completed", "completed", events.RiskLow, false}, {"failed", "failed", events.RiskHigh, false}, {"high-risk", "completed", events.RiskHigh, false}, {"pending-approval", "running", events.RiskMedium, true}, {"rollback-conflict", "completed", events.RiskMedium, false}, {"broken-integrity", "completed", events.RiskLow, false}}
	brokenID := ""
	for index, demo := range types {
		runID := fmt.Sprintf("demo-%s-%d", demo.name, now.Unix())
		run := events.Run{RunID: runID, AgentType: "codex", AgentID: "demo-agent", Status: "running", WorkspacePath: abs, StartedAt: now.Add(time.Duration(index) * time.Minute), Metadata: map[string]string{"repository": "demo/agent-audit-console", "git_branch": "feature/web-audit-v2", "git_commit_before": "1234567", "git_commit_after": "89abcde", "agent_version": "demo-v2"}}
		if err = store.CreateRun(ctx, run); err != nil {
			if !isDuplicate(err) {
				log.Fatal(err)
			}
			continue
		}
		appendEvent(store, runID, "action-plan", "", events.KindCommand, "Inspect repository", events.RiskLow, events.PolicyAllowed, events.ActionCompleted, map[string]any{"command": []string{"rg", "--files"}, "exit_code": 0, "stdout": "README.md\ninternal/api/server.go"}, run.StartedAt)
		appendEvent(store, runID, "action-change", "action-plan", events.KindFileChange, "Update audit console", demo.risk, events.PolicyAllowed, events.ActionCompleted, map[string]any{"path": "web/src/main.tsx", "change_type": "modified", "diff": "diff --git a/web/src/main.tsx b/web/src/main.tsx\n--- a/web/src/main.tsx\n+++ b/web/src/main.tsx\n@@ -1 +1 @@\n-old console\n+evidence-first console\n"}, run.StartedAt.Add(time.Second))
		policy := events.PolicyAllowed
		status := events.ActionCompleted
		if demo.approval {
			policy = events.PolicyPending
			status = events.ActionBlocked
		}
		appendEvent(store, runID, "action-test", "action-change", events.KindCommand, "Run verification", demo.risk, policy, status, map[string]any{"command": []string{"go", "test", "./..."}, "exit_code": map[bool]int{true: 1, false: 0}[demo.status == "failed"], "stdout": "ok internal/api", "stderr": map[bool]string{true: "FAIL demo package", false: ""}[demo.status == "failed"], "category": "test", "test": true}, run.StartedAt.Add(2*time.Second))
		if demo.approval {
			appendEvent(store, runID, "action-approval", "action-test", events.KindApproval, "Await operator approval", events.RiskMedium, events.PolicyPending, events.ActionBlocked, map[string]any{"request": "Approve protected operation"}, run.StartedAt.Add(3*time.Second))
		}
		if demo.name == "rollback-conflict" {
			addConflict(store, abs, runID, run.StartedAt)
		}
		if demo.status != "running" {
			if err = store.FinishRun(ctx, runID, demo.status, run.StartedAt.Add(4*time.Second)); err != nil {
				log.Fatal(err)
			}
		}
		if demo.name == "broken-integrity" {
			brokenID = runID
		}
	}
	largeID := fmt.Sprintf("demo-large-%d", now.Unix())
	largeRun := events.Run{RunID: largeID, AgentType: "load-test", AgentID: "demo-agent", Status: "running", WorkspacePath: abs, StartedAt: now.Add(10 * time.Minute), Metadata: map[string]string{"repository": "demo/large-timeline", "git_branch": "perf/10k"}}
	if err = store.CreateRun(ctx, largeRun); err == nil {
		for i := 0; i < *large; i++ {
			appendEvent(store, largeID, fmt.Sprintf("action-%05d", i/4), "", events.KindCommand, fmt.Sprintf("Recorded command %d", i), events.RiskLow, events.PolicyAllowed, events.ActionCompleted, map[string]any{"command": []any{"demo", "--index", i}, "exit_code": 0}, largeRun.StartedAt.Add(time.Duration(i)*time.Millisecond))
		}
		_ = store.FinishRun(ctx, largeID, "completed", largeRun.StartedAt.Add(time.Duration(*large)*time.Millisecond))
	}
	if err = store.Close(); err != nil {
		log.Fatal(err)
	}
	if brokenID != "" {
		db, openErr := sql.Open("sqlite", dbPath)
		if openErr != nil {
			log.Fatal(openErr)
		}
		if _, err = db.Exec(`UPDATE events SET intent='synthetic integrity break' WHERE run_id=? AND sequence=1`, brokenID); err != nil {
			log.Fatal(err)
		}
		_ = db.Close()
	}
	fmt.Printf("Demo audit data: %s\nLarge timeline events: %d\n", dbPath, *large)
}

func appendEvent(store *events.SQLiteStore, runID, actionID, parent string, kind events.Kind, intent, risk, policy string, status events.ActionStatus, data map[string]any, at time.Time) {
	raw, _ := json.Marshal(data)
	event := events.Event{EventID: events.NewID("event"), RunID: runID, ActionID: actionID, ParentActionID: parent, ActionStatus: status, Timestamp: at, Actor: events.Actor{Type: "agent", ID: "demo-agent"}, Kind: kind, Intent: intent, Evidence: events.Evidence{Data: raw}, Risk: events.Risk{Level: risk}, PolicyDecision: events.PolicyDecision{Status: policy, RuleID: "demo-policy", Reason: "Synthetic demonstration evidence"}, Reversibility: events.Reversibility{Status: events.Reversible}}
	if _, err := store.Append(context.Background(), event); err != nil {
		log.Fatal(err)
	}
}
func addConflict(store *events.SQLiteStore, dir, runID string, at time.Time) {
	path := filepath.Join(dir, "rollback-conflict.txt")
	_ = os.WriteFile(path, []byte("current modified content"), 0o600)
	target := []byte("original content")
	expected := []byte("expected post-action content")
	targetSum := sha256.Sum256(target)
	expectedSum := sha256.Sum256(expected)
	exists := true
	object := filepath.Join(dir, "synthetic-object")
	_ = os.WriteFile(object, target, 0o600)
	record := events.SnapshotRecord{SnapshotID: events.NewID("snapshot"), RunID: runID, ActionID: "action-change", FilePath: path, Exists: true, ExpectedExists: &exists, ContentHash: hex.EncodeToString(targetSum[:]), ExpectedHash: hex.EncodeToString(expectedSum[:]), ObjectPath: object, ByteSize: int64(len(target)), FileMode: 0o600, CapturedAt: at}
	if err := store.RecordSnapshot(context.Background(), record); err != nil {
		log.Fatal(err)
	}
}
func isDuplicate(err error) bool {
	return err != nil && (contains(err.Error(), "UNIQUE constraint") || contains(err.Error(), "constraint failed"))
}
func contains(value, part string) bool {
	for i := 0; i+len(part) <= len(value); i++ {
		if value[i:i+len(part)] == part {
			return true
		}
	}
	return false
}

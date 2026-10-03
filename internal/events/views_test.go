package events

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestRunOverviewsAndEvidenceFilters(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	run := Run{RunID: "run-overview", AgentType: "codex", AgentID: "agent-1", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{"repository": "owner/repo", "git_branch": "feature/audit"}}
	if err = store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	for i, item := range []struct {
		kind   Kind
		risk   string
		status ActionStatus
		data   string
	}{{KindCommand, RiskLow, ActionCompleted, `{"category":"test","test":true,"command":["go","test"]}`}, {KindFileChange, RiskHigh, ActionFailed, `{"path":"README.md","diff":"needle"}`}, {KindApproval, RiskMedium, ActionStarted, `{"request":"review"}`}} {
		e := Event{EventID: NewID("event"), RunID: run.RunID, ActionID: "action-" + string(rune('a'+i)), ActionStatus: item.status, Timestamp: time.Now().UTC(), Actor: Actor{Type: "agent", ID: "test"}, Kind: item.kind, Intent: "record needle", Evidence: Evidence{Data: json.RawMessage(item.data)}, Risk: Risk{Level: item.risk}, PolicyDecision: PolicyDecision{Status: map[bool]string{true: PolicyPending, false: PolicyAllowed}[item.kind == KindApproval]}, Reversibility: Reversibility{Status: Reversible}}
		if _, err = store.Append(ctx, e); err != nil {
			t.Fatal(err)
		}
	}
	items, err := store.ListRunOverviews(ctx, RunFilter{Repository: "owner/repo", Risk: RiskHigh, Search: "feature/audit", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].EventCount != 3 || items[0].ActionCount != 3 || items[0].Commands != 1 || items[0].Tests != 1 || items[0].FilesChanged != 1 || items[0].PendingApprovals != 1 {
		t.Fatalf("unexpected overview: %#v", items)
	}
	filtered, err := store.Query(ctx, Query{RunID: run.RunID, RiskLevels: []string{RiskHigh}, ActionStatuses: []ActionStatus{ActionFailed}, Search: "README", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 1 || filtered[0].Kind != KindFileChange {
		t.Fatalf("unexpected filter result: %#v", filtered)
	}
}

func TestRunOverviewCursorCapsLargeRunList(t *testing.T) {
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	start := time.Now().UTC()
	for i := 0; i < 120; i++ {
		run := Run{RunID: NewID("run"), AgentType: "load", AgentID: "agent", Status: "completed", WorkspacePath: t.TempDir(), StartedAt: start.Add(time.Duration(i) * time.Millisecond), Metadata: map[string]string{}}
		if err = store.CreateRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	first, err := store.ListRunOverviews(ctx, RunFilter{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 51 {
		t.Fatalf("limit+1=%d", len(first))
	}
	last := first[49].Run
	cursor := last.StartedAt.UTC().Format(time.RFC3339Nano) + "|" + last.RunID
	second, err := store.ListRunOverviews(ctx, RunFilter{Limit: 50, Cursor: cursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second) == 0 || second[0].Run.RunID == last.RunID {
		t.Fatalf("cursor did not advance")
	}
}

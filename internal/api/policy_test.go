package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/auth"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/agent-audit-console/agent-audit-console/internal/policy"
)

func TestPolicyViewerSimulationValidationAndHistoricalExplain(t *testing.T) {
	dir := t.TempDir()
	store, err := events.OpenSQLite(filepath.Join(dir, "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	policyPath := filepath.Join(dir, "policy.yaml")
	if err = os.WriteFile(policyPath, []byte("version: 1\ndefault: {risk: low, decision: allow}\nrules:\n- {id: push, command_regex: git.push, risk: high, decision: require_approval}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := policy.Load(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	accessPath := filepath.Join(dir, "access.yaml")
	token, err := auth.AddToken(accessPath, "viewer", auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	manager, _ := auth.Load(accessPath)
	run := events.Run{RunID: "run-policy", AgentType: "test", AgentID: "test", Status: "running", WorkspacePath: dir, StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err = store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	evidence := json.RawMessage(`{"command":["git","push"]}`)
	event := events.Event{EventID: "event-policy", RunID: run.RunID, ActionID: "action-policy", Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "test"}, Kind: events.KindCommand, Intent: "push", Evidence: events.Evidence{Data: evidence}, Risk: events.Risk{Level: events.RiskHigh}, PolicyDecision: events.PolicyDecision{Status: events.PolicyPending, PolicyFingerprint: engine.Fingerprint()}, Reversibility: events.Reversibility{Status: events.Irreversible}}
	if _, err = store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewWithPolicy(store, engine, manager))
	defer server.Close()
	for _, item := range []struct{ method, path, body string }{{"GET", "/api/policy", ""}, {"POST", "/api/policy/simulate", `{"action":"push","command":["git","push"]}`}, {"POST", "/api/policy/validate", `{"document":"version: 1\ndefault: {risk: low, decision: allow}\nrules: []\n"}`}, {"GET", "/api/runs/run-policy/actions/action-policy/policy", ""}} {
		req, _ := http.NewRequest(item.method, server.URL+item.path, bytes.NewBufferString(item.body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		response, requestErr := http.DefaultClient.Do(req)
		if requestErr != nil {
			t.Fatal(requestErr)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", item.path, response.StatusCode)
		}
	}
}

func TestPolicyAPIBodyLimit(t *testing.T) {
	dir := t.TempDir()
	store, _ := events.OpenSQLite(filepath.Join(dir, "audit.db"))
	defer store.Close()
	path := filepath.Join(dir, "policy.yaml")
	_ = os.WriteFile(path, policy.DefaultRules, 0o600)
	engine, _ := policy.Load(path)
	server := httptest.NewServer(NewWithPolicy(store, engine, nil))
	defer server.Close()
	request, _ := http.NewRequest(http.MethodPost, server.URL+"/api/policy/simulate", bytes.NewBuffer(make([]byte, 65<<10)))
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status=%d", response.StatusCode)
	}
}

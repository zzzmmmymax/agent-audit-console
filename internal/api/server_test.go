package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/auth"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func TestServerRunsAndStaticUI(t *testing.T) {
	store, err := events.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run := events.Run{RunID: "run-api", AgentType: "test", AgentID: "api", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(store))
	defer server.Close()
	for _, path := range []string{"/", "/healthz", "/api/runs", "/api/runs/run-api"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, response.StatusCode)
		}
	}
}

func TestWebAuditV2OverviewVerifyAndFilters(t *testing.T) {
	store, err := events.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	ctx := context.Background()
	run := events.Run{RunID: "run-web-v2", AgentType: "codex", AgentID: "agent", Status: "running", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{"repository": "owner/repo"}}
	if err = store.CreateRun(ctx, run); err != nil {
		t.Fatal(err)
	}
	event := events.Event{EventID: "event-v2", RunID: run.RunID, ActionID: "action-v2", ActionStatus: events.ActionFailed, Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "agent"}, Kind: events.KindCommand, Intent: "run secure tests", Evidence: events.Evidence{Data: json.RawMessage(`{"command":["go","test"],"category":"test"}`)}, Risk: events.Risk{Level: events.RiskHigh}, PolicyDecision: events.PolicyDecision{Status: events.PolicyAllowed}, Reversibility: events.Reversibility{Status: events.Irreversible}}
	if _, err = store.Append(ctx, event); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(store))
	defer server.Close()
	for _, path := range []string{"/api/run-overviews?repository=owner%2Frepo&risk=high&q=owner", "/api/runs/run-web-v2?risk=high&status=failed&q=secure", "/api/runs/run-web-v2/rollback-preview"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, response.StatusCode)
		}
	}
	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/runs/run-web-v2/verify", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var result map[string]any
	if err = json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "verified" {
		t.Fatalf("verify=%v", result)
	}
}

func TestServerBearerAuthentication(t *testing.T) {
	store, err := events.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	accessPath := filepath.Join(t.TempDir(), "access.yaml")
	token, err := auth.AddToken(accessPath, "test-viewer", auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := auth.Load(accessPath)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewWithAuth(store, manager))
	defer server.Close()

	response, err := http.Get(server.URL + "/api/runs")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status=%d", response.StatusCode)
	}

	request, err := http.NewRequest(http.MethodGet, server.URL+"/api/whoami", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err = http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status=%d", response.StatusCode)
	}

	response, err = http.Get(server.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health status=%d", response.StatusCode)
	}
}

func TestRBACAndRollbackRequest(t *testing.T) {
	store, err := events.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run := events.Run{RunID: "run-rbac", AgentType: "custom", AgentID: "test", Status: "completed", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	event := events.Event{EventID: "event-rbac", RunID: run.RunID, ActionID: "action-rbac", Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "test"}, Kind: events.KindCommand, Intent: "test", Evidence: events.Evidence{Data: []byte(`{"ok":true}`)}, Risk: events.Risk{Level: events.RiskLow}, PolicyDecision: events.PolicyDecision{Status: events.PolicyAllowed}, Reversibility: events.Reversibility{Status: events.Irreversible}}
	if _, err := store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "access.yaml")
	viewer, err := auth.AddToken(path, "viewer", auth.RoleViewer)
	if err != nil {
		t.Fatal(err)
	}
	operator, err := auth.AddToken(path, "operator", auth.RoleOperator)
	if err != nil {
		t.Fatal(err)
	}
	manager, err := auth.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewWithAuth(store, manager))
	defer server.Close()
	body := []byte(`{"target_event_id":"event-rbac","reason":"review"}`)
	for _, item := range []struct {
		token string
		want  int
	}{{viewer, http.StatusForbidden}, {operator, http.StatusAccepted}} {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/api/runs/run-rbac/rollback-requests", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+item.token)
		request.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != item.want {
			t.Fatalf("role status=%d want=%d", response.StatusCode, item.want)
		}
	}
}

func TestRunEventsArePaginatedAndReady(t *testing.T) {
	store, err := events.OpenSQLite(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	run := events.Run{RunID: "run-pages", AgentType: "custom", AgentID: "test", Status: "completed", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		event := events.Event{EventID: events.NewID("event"), RunID: run.RunID, ActionID: "action-pages", Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "test"}, Kind: events.KindCommand, Intent: "test", Evidence: events.Evidence{Data: []byte(`{"ok":true}`)}, Risk: events.Risk{Level: events.RiskLow}, PolicyDecision: events.PolicyDecision{Status: events.PolicyAllowed}, Reversibility: events.Reversibility{Status: events.Irreversible}}
		if _, err := store.Append(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}
	server := httptest.NewServer(New(store))
	defer server.Close()
	for _, path := range []string{"/readyz", "/api/runs/run-pages?limit=2"} {
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s status=%d", path, response.StatusCode)
		}
	}
}

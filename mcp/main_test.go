package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPStartRunAndSummary(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := newServer(service)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "start_run", Arguments: map[string]any{"agent_type": "custom", "agent_id": "mcp-test", "workspace_path": filepath.Clean(t.TempDir())}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool error: %+v", result.Content)
	}
	tools := session.Tools(context.Background(), nil)
	count := 0
	for tool, err := range tools {
		if err != nil {
			t.Fatal(err)
		}
		if tool.Name != "" {
			count++
		}
	}
	if count != 21 {
		t.Fatalf("tools=%d", count)
	}
}

func TestMCPActionLifecycleContextAndIdempotency(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	start, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "s1", WorkspacePath: t.TempDir()}, AgentID: "test", IdempotencyKey: "start-1"})
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "s1", WorkspacePath: t.TempDir()}, AgentID: "test", IdempotencyKey: "start-1"})
	if err != nil || !replayed.Replayed || replayed.RunID != start.RunID {
		t.Fatalf("replay=%+v err=%v", replayed, err)
	}
	input := actionInput{runRef: runRef{RunID: start.RunID}, ActionID: "a1", CorrelationID: "c1", Kind: "mcp_call", Intent: "test", IdempotencyKey: "action-1"}
	first, err := writeAction(ctx, service, input, events.ActionPlanned)
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeAction(ctx, service, input, events.ActionPlanned)
	if err != nil || !second.Replayed || second.EventID != first.EventID {
		t.Fatalf("replay=%+v err=%v", second, err)
	}
	if err := service.Store.VerifyRun(ctx, start.RunID); err != nil {
		t.Fatal(err)
	}
}

func TestMCPIntegrationDemo(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := newServer(service).Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "integration-demo", Version: "2"}, nil)
	session, err := client.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		result, callErr := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if callErr != nil || result.IsError {
			t.Fatalf("%s err=%v result=%+v", name, callErr, result)
		}
		return result
	}
	call("health", map[string]any{})
	call("capabilities", map[string]any{})
	started := call("start_run", map[string]any{"agent_type": "custom", "agent_id": "demo", "session_id": "demo-session", "workspace_path": t.TempDir()})
	raw, _ := json.Marshal(started.StructuredContent)
	var startedValue struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(raw, &startedValue)
	if startedValue.RunID == "" {
		t.Fatalf("start output=%s", raw)
	}
	call("record_action", map[string]any{"run_id": startedValue.RunID, "action_id": "demo-action", "correlation_id": "demo-chain", "kind": "mcp_call", "intent": "run integration demo"})
	call("complete_action", map[string]any{"run_id": startedValue.RunID, "action_id": "demo-action", "correlation_id": "demo-chain", "kind": "mcp_call", "intent": "complete integration demo"})
	call("get_run_summary", map[string]any{"run_id": startedValue.RunID, "compact": true})
	if err := service.Store.VerifyRun(context.Background(), startedValue.RunID); err != nil {
		t.Fatal(err)
	}
}

func TestApprovalRequestCannotSelfApprove(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	run, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "approval", WorkspacePath: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := requestApproval(ctx, service, approvalInput{actionInput: actionInput{runRef: runRef{RunID: run.RunID}, ActionID: "approval-action"}, Reason: "high risk"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "pending" {
		t.Fatalf("status=%s", out.Status)
	}
	items, err := service.Store.Query(ctx, events.Query{RunID: run.RunID, Kinds: []events.Kind{events.KindApproval}, Limit: 10})
	if err != nil || len(items) != 1 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if items[0].PolicyDecision.Status != events.PolicyPending {
		t.Fatalf("decision=%s", items[0].PolicyDecision.Status)
	}
}

func TestAutomaticContextDoesNotCollideWithoutSession(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	workspace := t.TempDir()
	a, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", WorkspacePath: workspace}})
	if err != nil {
		t.Fatal(err)
	}
	b, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", WorkspacePath: workspace}})
	if err != nil {
		t.Fatal(err)
	}
	if a.ContextKey == b.ContextKey || a.RunID == b.RunID {
		t.Fatalf("contexts collided: %+v %+v", a, b)
	}
	if got, err := resolveRun(ctx, service, runRef{ContextKey: a.ContextKey}); err != nil || got != a.RunID {
		t.Fatalf("resolved=%s err=%v", got, err)
	}
}

func TestActionRejectsOversizedPayload(t *testing.T) {
	service, err := audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	run, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "large", WorkspacePath: t.TempDir()}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = writeAction(ctx, service, actionInput{runRef: runRef{RunID: run.RunID}, Kind: events.KindMCPCall, Intent: "large", Evidence: map[string]any{"data": strings.Repeat("x", (1<<20)+1)}}, events.ActionStarted)
	if err == nil {
		t.Fatal("oversized payload accepted")
	}
}

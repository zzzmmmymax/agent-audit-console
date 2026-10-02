package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
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
	if count != 4 {
		t.Fatalf("tools=%d", count)
	}
}

package integrations

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONMergePreservesServersAndBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	if err := os.WriteFile(path, []byte(`{"theme":"dark","mcpServers":{"other":{"command":"x"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	command := filepath.Join(t.TempDir(), "audit-mcp")
	_ = os.WriteFile(command, []byte("test"), 0700)
	plan, _ := BuildPlan(Cursor, path, command, []string{"--data-dir", "x"})
	result, err := Apply(plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.BackupPath == "" {
		t.Fatal("backup missing")
	}
	var root map[string]any
	data, _ := os.ReadFile(path)
	if err = json.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	servers := root["mcpServers"].(map[string]any)
	if servers["other"] == nil || servers["agent-audit"] == nil {
		t.Fatal("merge overwrote servers")
	}
}

func TestMalformedConfigIsNotModified(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp.json")
	before := []byte(`{"bad"`)
	_ = os.WriteFile(path, before, 0600)
	command := filepath.Join(t.TempDir(), "audit-mcp")
	_ = os.WriteFile(command, []byte("test"), 0700)
	plan, _ := BuildPlan(Cursor, path, command, nil)
	if _, err := Apply(plan); err == nil {
		t.Fatal("malformed config accepted")
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(before) {
		t.Fatal("malformed config modified")
	}
}

func TestTOMLMergePreservesContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	_ = os.WriteFile(path, []byte("model = \"x\"\n[mcp_servers.other]\ncommand = \"other\"\n"), 0600)
	command := filepath.Join(t.TempDir(), "audit-mcp")
	_ = os.WriteFile(command, []byte("test"), 0700)
	plan, _ := BuildPlan(Codex, path, command, nil)
	if _, err := Apply(plan); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	text := string(data)
	if !strings.Contains(text, "mcp_servers.other") || !strings.Contains(text, "mcp_servers.agent-audit") {
		t.Fatal(text)
	}
}

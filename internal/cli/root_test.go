package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/buildinfo"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func TestVersionAndCommandHelp(t *testing.T) {
	original := buildinfo.Version
	buildinfo.Version = "v0.1.0-test"
	defer func() { buildinfo.Version = original }()
	var stdout, stderr bytes.Buffer
	root := New(&stdout, &stderr)
	root.SetArgs([]string{"version"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Agent Audit Console") || !strings.Contains(stdout.String(), "v0.1.0-test") {
		t.Fatalf("version output=%q", stdout.String())
	}
	for _, name := range []string{"run", "log", "show", "restore", "export", "access-token", "sync", "verify", "doctor", "version"} {
		command, _, err := root.Find([]string{name})
		if err != nil || command.Short == "" {
			t.Fatalf("command %s missing help: %v", name, err)
		}
	}
}

func TestVerifyAndDoctor(t *testing.T) {
	dataDir := t.TempDir()
	service, err := audit.Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	run := events.Run{RunID: "run-cli-verify", AgentType: "custom", AgentID: "test", Status: "completed", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := service.Store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	event := events.Event{EventID: "event-cli", RunID: run.RunID, ActionID: "action-cli", Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "test"}, Kind: events.KindCommand, Intent: "test", Evidence: events.Evidence{Data: []byte(`{"ok":true}`)}, Risk: events.Risk{Level: events.RiskLow}, PolicyDecision: events.PolicyDecision{Status: events.PolicyAllowed}, Reversibility: events.Reversibility{Status: events.Irreversible}}
	if _, err := service.Store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	service.Close()
	for _, args := range [][]string{{"--data-dir", dataDir, "verify", run.RunID}, {"--data-dir", dataDir, "doctor"}} {
		var stdout, stderr bytes.Buffer
		root := New(&stdout, &stderr)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("args=%v err=%v output=%s", args, err, stdout.String())
		}
		if strings.Contains(strings.ToLower(stdout.String()), "token=") {
			t.Fatal("doctor leaked token-shaped output")
		}
	}
}

func TestAtomicOutputPreservesExistingFileOnFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected := errors.New("write failed")
	err := atomicOutput(path, func(file *os.File) error {
		_, _ = file.WriteString("partial")
		return expected
	})
	if !errors.Is(err, expected) {
		t.Fatalf("err=%v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("destination=%q", data)
	}
}

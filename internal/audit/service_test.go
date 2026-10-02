package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func TestCommandHelper(t *testing.T) {
	if os.Getenv("GO_WANT_AUDIT_HELPER") != "1" {
		return
	}
	separator := 0
	for i, value := range os.Args {
		if value == "--" {
			separator = i + 1
			break
		}
	}
	paths := os.Args[separator:]
	if len(paths) != 3 {
		os.Exit(2)
	}
	if err := os.WriteFile(paths[0], []byte("after\n"), 0o644); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(paths[1], []byte("new\n"), 0o644); err != nil {
		os.Exit(4)
	}
	if err := os.Remove(paths[2]); err != nil {
		os.Exit(5)
	}
	os.Exit(0)
}

func TestApprovalHelper(t *testing.T) {
	if os.Getenv("GO_WANT_APPROVAL_HELPER") != "1" {
		return
	}
	if err := os.WriteFile(os.Args[len(os.Args)-1], []byte("executed"), 0o644); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestModifyOneHelper(t *testing.T) {
	if os.Getenv("GO_WANT_MODIFY_ONE_HELPER") != "1" {
		return
	}
	if err := os.WriteFile(os.Args[len(os.Args)-1], []byte("after\n"), 0o644); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

func TestSecretOutputHelper(t *testing.T) {
	if os.Getenv("GO_WANT_SECRET_OUTPUT_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, "Authorization: Bearer ghp_abcdefghijklmnopqrstuvwxyz123456")
	fmt.Fprintln(os.Stderr, "OPENAI_API_KEY=sk-abcdefghijklmnopqrstuvwxyz")
	os.Exit(0)
}

func TestRunCommandCapturesAndRestoresFileChanges(t *testing.T) {
	workspace := t.TempDir()
	dataDir := t.TempDir()
	modified := filepath.Join(workspace, "modified.txt")
	added := filepath.Join(workspace, "added.txt")
	deleted := filepath.Join(workspace, "deleted.txt")
	if err := os.WriteFile(modified, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deleted, []byte("deleted-before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Setenv("GO_WANT_AUDIT_HELPER", "1")
	result, err := service.RunCommand(context.Background(), RunOptions{Command: []string{os.Args[0], "-test.run=TestCommandHelper", "--", modified, added, deleted}, Workdir: workspace})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 || result.Changes != 3 {
		t.Fatalf("result=%+v", result)
	}
	if _, err := service.Restore(context.Background(), modified); err != nil {
		t.Fatal(err)
	}
	assertContent(t, modified, "before\n")
	if _, err := service.Restore(context.Background(), deleted); err != nil {
		t.Fatal(err)
	}
	assertContent(t, deleted, "deleted-before\n")
	if _, err := service.Restore(context.Background(), added); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(added); !os.IsNotExist(err) {
		t.Fatalf("added file still exists: %v", err)
	}
	if err := service.Store.VerifyRun(context.Background(), result.Run.RunID); err != nil {
		t.Fatal(err)
	}
}

func TestHighRiskCommandIsRejectedBeforeExecution(t *testing.T) {
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	result, err := service.RunCommand(context.Background(), RunOptions{Command: []string{"git", "push"}, Workdir: t.TempDir()})
	if !errors.Is(err, ErrApprovalRejected) {
		t.Fatalf("err=%v", err)
	}
	if result.Run.Status != "cancelled" {
		t.Fatalf("status=%s", result.Run.Status)
	}
	items, err := service.Store.Query(context.Background(), events.Query{RunID: result.Run.RunID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Kind != events.KindApproval || items[0].PolicyDecision.Status != events.PolicyPending || items[1].PolicyDecision.Status != events.PolicyRejected {
		t.Fatalf("events=%+v", items)
	}
}

func TestApprovedHighRiskCommandExecutes(t *testing.T) {
	dataDir := t.TempDir()
	rules := `version: 1
default: {risk: low, decision: allowed}
rules:
  - id: test-high-risk
    description: test approval
    command_regex: 'TestApprovalHelper'
    risk: high
    decision: pending
`
	if err := os.WriteFile(filepath.Join(dataDir, "policy.yaml"), []byte(rules), 0o600); err != nil {
		t.Fatal(err)
	}
	service, err := Open(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	marker := filepath.Join(t.TempDir(), "marker.txt")
	t.Setenv("GO_WANT_APPROVAL_HELPER", "1")
	result, err := service.RunCommand(context.Background(), RunOptions{Command: []string{os.Args[0], "-test.run=TestApprovalHelper", "--", marker}, Workdir: t.TempDir(), Approve: func(ApprovalRequest) bool { return true }})
	if err != nil {
		t.Fatal(err)
	}
	if result.Run.Status != "completed" {
		t.Fatalf("status=%s", result.Run.Status)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	items, err := service.Store.Query(context.Background(), events.Query{RunID: result.Run.RunID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 3 || items[0].PolicyDecision.Status != events.PolicyPending || items[1].PolicyDecision.Status != events.PolicyApproved || items[2].Kind != events.KindCommand {
		t.Fatalf("events=%+v", items)
	}
}

func TestExportJSONAndHTML(t *testing.T) {
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	run := events.Run{RunID: "run-export", AgentType: "test", AgentID: "test", Status: "completed", WorkspacePath: t.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := service.Store.CreateRun(context.Background(), run); err != nil {
		t.Fatal(err)
	}
	event := newEvent(run.RunID, "action-export", events.KindCommand, "test export", json.RawMessage(`{"ok":true}`), events.NotApplicable)
	if _, err := service.Store.Append(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	for _, format := range []string{"json", "html"} {
		var output bytes.Buffer
		if err := service.ExportRun(context.Background(), run.RunID, format, &output); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(output.String(), run.RunID) {
			t.Fatalf("%s export missing run", format)
		}
	}
}

func TestRestoreRejectsConflictUnlessForced(t *testing.T) {
	workspace := t.TempDir()
	path := filepath.Join(workspace, "conflict.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Setenv("GO_WANT_MODIFY_ONE_HELPER", "1")
	if _, err := service.RunCommand(context.Background(), RunOptions{Command: []string{os.Args[0], "-test.run=TestModifyOneHelper", "--", path}, Workdir: workspace}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("user edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Restore(context.Background(), path); !errors.Is(err, ErrRestoreConflict) {
		t.Fatalf("expected conflict, got %v", err)
	}
	assertContent(t, path, "user edit\n")
	if _, err := service.RestoreWithOptions(context.Background(), path, RestoreOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	assertContent(t, path, "before\n")
}

func TestRestoreRejectsSymlinkParentEscape(t *testing.T) {
	workspace := t.TempDir()
	directory := filepath.Join(workspace, "nested")
	if err := os.Mkdir(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "file.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Setenv("GO_WANT_MODIFY_ONE_HELPER", "1")
	if _, err := service.RunCommand(context.Background(), RunOptions{Command: []string{os.Args[0], "-test.run=TestModifyOneHelper", "--", path}, Workdir: workspace}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(directory); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, directory); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	if _, err := service.RestoreWithOptions(context.Background(), path, RestoreOptions{Force: true}); err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected symlink escape rejection, got %v", err)
	}
}

func TestSecretsDoNotReachEventsOrExports(t *testing.T) {
	service, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	t.Setenv("GO_WANT_SECRET_OUTPUT_HELPER", "1")
	githubSecret := "ghp_abcdefghijklmnopqrstuvwxyz123456"
	openAISecret := "sk-abcdefghijklmnopqrstuvwxyz"
	result, err := service.RunCommand(context.Background(), RunOptions{Command: []string{os.Args[0], "-test.run=TestSecretOutputHelper", "--", "--token=" + githubSecret}, Workdir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	var exported bytes.Buffer
	if err := service.ExportRun(context.Background(), result.Run.RunID, "json", &exported); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{githubSecret, openAISecret} {
		if strings.Contains(exported.String(), secret) {
			t.Fatalf("secret leaked to export: %s", secret)
		}
	}
}

func TestCaptureBufferIsBounded(t *testing.T) {
	buffer := newCaptureBuffer(16)
	input := strings.Repeat("x", 1024)
	written, err := buffer.Write([]byte(input))
	if err != nil || written != len(input) {
		t.Fatalf("written=%d err=%v", written, err)
	}
	if !strings.Contains(buffer.String(), "OUTPUT TRUNCATED") || len(buffer.String()) > 100 {
		t.Fatalf("buffer=%q", buffer.String())
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("content=%q want=%q", data, want)
	}
}

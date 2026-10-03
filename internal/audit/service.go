package audit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/adapters"
	"github.com/agent-audit-console/agent-audit-console/internal/capture"
	runtimeconfig "github.com/agent-audit-console/agent-audit-console/internal/config"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/agent-audit-console/agent-audit-console/internal/policy"
	"github.com/agent-audit-console/agent-audit-console/internal/redact"
	"github.com/agent-audit-console/agent-audit-console/internal/snapshots"
)

type Service struct {
	Store     *events.SQLiteStore
	Snapshots *snapshots.DiskStore
	Policy    *policy.Engine
	Redactor  *redact.Redactor
	Adapters  *adapters.Registry
	DataDir   string
}

var ErrApprovalRejected = errors.New("high-risk action was not approved")
var ErrRestoreConflict = errors.New("restore conflict")

type ApprovalRequest struct {
	RunID, ActionID string
	Risk            events.Risk
	Decision        events.PolicyDecision
	Command         []string
}

func DefaultDataDir() string {
	if value := os.Getenv("AGENT_AUDIT_HOME"); value != "" {
		return value
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return ".agent-audit"
	}
	return filepath.Join(base, "agent-audit-console")
}

func Open(dataDir string) (*Service, error) {
	resolved, err := runtimeconfig.Load("")
	if err != nil {
		return nil, err
	}
	if dataDir != "" {
		resolved, err = resolved.WithDataDir(dataDir)
		if err != nil {
			return nil, err
		}
	}
	return OpenWithConfig(resolved)
}

func OpenWithConfig(resolved runtimeconfig.Runtime) (*Service, error) {
	var err error
	resolved, err = resolved.Finalize()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(resolved.DataDir, 0o700); err != nil {
		return nil, err
	}
	store, err := events.OpenSQLite(resolved.DatabasePath)
	if err != nil {
		return nil, err
	}
	objects, err := snapshots.NewDiskStore(resolved.SnapshotDir)
	if err != nil {
		store.Close()
		return nil, err
	}
	policyPath := resolved.PolicyFile
	if err := policy.EnsureDefault(policyPath); err != nil {
		store.Close()
		return nil, err
	}
	engine, err := policy.LoadLayered(policyPath, resolved.TeamPolicies...)
	if err != nil {
		store.Close()
		return nil, err
	}
	return &Service{Store: store, Snapshots: objects, Policy: engine, Redactor: redact.New(), Adapters: adapters.Builtins(), DataDir: resolved.DataDir}, nil
}
func (s *Service) Close() error { return s.Store.Close() }

func (s *Service) Ready(ctx context.Context) error {
	if err := s.Store.Ready(ctx); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	if err := s.Snapshots.Ready(ctx); err != nil {
		return fmt.Errorf("snapshots: %w", err)
	}
	if err := s.Policy.Ready(); err != nil {
		return fmt.Errorf("policy: %w", err)
	}
	return nil
}

type RunOptions struct {
	Command                     []string
	Workdir, AgentType, AgentID string
	Stdout, Stderr              io.Writer
	Approve                     func(ApprovalRequest) bool
}
type RunResult struct {
	Run      events.Run
	ExitCode int
	Changes  int
}

func (s *Service) RunCommand(ctx context.Context, options RunOptions) (RunResult, error) {
	if len(options.Command) == 0 {
		return RunResult{}, errors.New("a command is required after --")
	}
	workdir := options.Workdir
	if workdir == "" {
		workdir, _ = os.Getwd()
	}
	absolute, err := filepath.Abs(workdir)
	if err != nil {
		return RunResult{}, err
	}
	if options.AgentType == "" {
		options.AgentType = "codex"
	}
	if options.AgentID == "" {
		options.AgentID = "audit"
	}
	agentAdapter, err := s.Adapters.Resolve(options.AgentType)
	if err != nil {
		return RunResult{}, err
	}
	options.AgentType = agentAdapter.Descriptor().Name
	started := time.Now().UTC()
	run := events.Run{RunID: events.NewID("run"), AgentType: options.AgentType, AgentID: options.AgentID, Status: "running", WorkspacePath: absolute, StartedAt: started, Metadata: map[string]string{"command": strings.Join(s.Redactor.Strings(options.Command), " ")}}
	if err := s.Store.CreateRun(ctx, run); err != nil {
		return RunResult{}, err
	}
	actionID := events.NewID("action")
	plannedEvidence, err := json.Marshal(map[string]any{"command": options.Command, "working_directory": absolute})
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	planned, err := agentAdapter.Normalize(adapters.Action{RunID: run.RunID, ActionID: actionID, ActorID: options.AgentID, Kind: events.KindCommand, Intent: "execute command", Evidence: events.Evidence{Data: plannedEvidence}, Reversibility: events.Reversibility{Status: events.Irreversible}})
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	risk, decision, err := s.Policy.Evaluate(ctx, planned)
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	if decision.Status == events.PolicyPending {
		if err := s.recordApproval(ctx, run.RunID, actionID, risk, decision, events.PolicyPending); err != nil {
			return RunResult{}, s.failRun(ctx, run.RunID, err)
		}
		approved := options.Approve != nil && options.Approve(ApprovalRequest{RunID: run.RunID, ActionID: actionID, Risk: risk, Decision: decision, Command: options.Command})
		status := events.PolicyRejected
		if approved {
			status = events.PolicyApproved
		}
		if err := s.recordApproval(ctx, run.RunID, actionID, risk, decision, status); err != nil {
			return RunResult{}, s.failRun(ctx, run.RunID, err)
		}
		if !approved {
			ended := time.Now().UTC()
			_ = s.Store.FinishRun(ctx, run.RunID, "cancelled", ended)
			run.Status = "cancelled"
			run.EndedAt = &ended
			return RunResult{Run: run, ExitCode: -1}, ErrApprovalRejected
		}
		decision.Status = events.PolicyApproved
	}
	before, err := capture.ScanWorkspace(ctx, absolute, s.Snapshots)
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	stdout, stderr := newCaptureBuffer(1<<20), newCaptureBuffer(1<<20)
	out := io.MultiWriter(stdout)
	errOut := io.MultiWriter(stderr)
	if options.Stdout != nil {
		out = io.MultiWriter(options.Stdout, stdout)
	}
	if options.Stderr != nil {
		errOut = io.MultiWriter(options.Stderr, stderr)
	}
	command := exec.CommandContext(ctx, options.Command[0], options.Command[1:]...)
	command.Dir = absolute
	command.Stdout = out
	command.Stderr = errOut
	execStarted := time.Now().UTC()
	runErr := command.Run()
	finished := time.Now().UTC()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}
	commandEvidence, err := json.Marshal(map[string]any{"command": s.Redactor.Strings(options.Command), "working_directory": absolute, "started_at": execStarted, "ended_at": finished, "duration_ms": finished.Sub(execStarted).Milliseconds(), "exit_code": exitCode, "stdout": s.Redactor.String(stdout.String()), "stderr": s.Redactor.String(stderr.String())})
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	commandEvent := newEvent(run.RunID, actionID, events.KindCommand, "execute command", commandEvidence, events.Irreversible)
	commandEvent.Actor = planned.Actor
	commandEvent.Risk, commandEvent.PolicyDecision = risk, decision
	if _, err := s.Store.Append(ctx, commandEvent); err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	if strings.EqualFold(filepath.Base(options.Command[0]), "git") || strings.EqualFold(filepath.Base(options.Command[0]), "git.exe") {
		gitEvent := newEvent(run.RunID, actionID, events.KindGit, "execute git operation", commandEvidence, events.PartlyReversible)
		gitEvent.Actor = planned.Actor
		gitEvent.Risk, gitEvent.PolicyDecision = risk, decision
		if _, err := s.Store.Append(ctx, gitEvent); err != nil {
			return RunResult{}, s.failRun(ctx, run.RunID, err)
		}
	}
	after, scanErr := capture.ScanWorkspace(ctx, absolute, s.Snapshots)
	if scanErr != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, scanErr)
	}
	changes, err := capture.CompareWorkspace(before, after)
	if err != nil {
		return RunResult{}, s.failRun(ctx, run.RunID, err)
	}
	for _, change := range changes {
		if err := s.recordChange(ctx, run.RunID, actionID, planned.Actor, change); err != nil {
			return RunResult{}, s.failRun(ctx, run.RunID, err)
		}
	}
	status := "completed"
	if exitCode != 0 {
		status = "failed"
	}
	if err := s.Store.FinishRun(ctx, run.RunID, status, finished); err != nil {
		return RunResult{}, err
	}
	run.Status = status
	run.EndedAt = &finished
	return RunResult{Run: run, ExitCode: exitCode, Changes: len(changes)}, nil
}

func (s *Service) recordChange(ctx context.Context, runID, actionID string, actor events.Actor, change capture.Change) error {
	now := time.Now().UTC()
	snapshot := events.SnapshotRecord{SnapshotID: events.NewID("snapshot"), RunID: runID, ActionID: actionID, CapturedAt: now}
	path := ""
	beforeHash := ""
	afterHash := ""
	mode := uint32(0)
	if change.Before != nil {
		path = change.Before.Path
		beforeHash = change.Before.Hash
		mode = uint32(change.Before.Mode)
		snapshot.Exists = true
		snapshot.FilePath = path
		snapshot.ContentHash = change.Before.Hash
		snapshot.ObjectPath = change.Before.ObjectPath
		snapshot.ByteSize = change.Before.Size
		snapshot.FileMode = mode
	} else {
		path = change.After.Path
		snapshot.FilePath = path
		snapshot.Exists = false
	}
	if change.After != nil {
		afterHash = change.After.Hash
		if mode == 0 {
			mode = uint32(change.After.Mode)
		}
	}
	expectedExists := change.After != nil
	snapshot.ExpectedExists = &expectedExists
	snapshot.ExpectedHash = afterHash
	if err := s.Store.RecordSnapshot(ctx, snapshot); err != nil {
		return err
	}
	evidence, err := json.Marshal(map[string]any{"change_type": change.Type, "path": path, "before_hash": beforeHash, "after_hash": afterHash, "diff": s.Redactor.String(change.Diff), "snapshot_id": snapshot.SnapshotID})
	if err != nil {
		return err
	}
	event := newEvent(runID, actionID, events.KindFileChange, "capture workspace change", evidence, events.Reversible)
	event.Actor = actor
	_, err = s.Store.Append(ctx, event)
	return err
}

func (s *Service) recordApproval(ctx context.Context, runID, actionID string, risk events.Risk, decision events.PolicyDecision, status string) error {
	decision.Status = status
	evidence, err := json.Marshal(map[string]any{"rule_id": decision.RuleID, "reason": decision.Reason, "status": status})
	if err != nil {
		return err
	}
	event := newEvent(runID, actionID, events.KindApproval, "review high-risk action", evidence, events.NotApplicable)
	event.Actor = events.Actor{Type: "user", ID: "local-user"}
	event.Risk, event.PolicyDecision = risk, decision
	_, err = s.Store.Append(ctx, event)
	return err
}

func newEvent(runID, actionID string, kind events.Kind, intent string, evidence json.RawMessage, reversibility string) events.Event {
	return events.Event{EventID: events.NewID("event"), RunID: runID, ActionID: actionID, Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: "audit-cli"}, Kind: kind, Intent: intent, Evidence: events.Evidence{Data: evidence}, Risk: events.Risk{Level: events.RiskUnknown}, PolicyDecision: events.PolicyDecision{Status: events.PolicyNotEvaluated}, Reversibility: events.Reversibility{Status: reversibility}}
}
func (s *Service) failRun(ctx context.Context, id string, cause error) error {
	_ = s.Store.FinishRun(ctx, id, "failed", time.Now().UTC())
	return cause
}

type RestoreOptions struct{ Force bool }

func (s *Service) Restore(ctx context.Context, path string) (events.RollbackRecord, error) {
	return s.RestoreWithOptions(ctx, path, RestoreOptions{})
}

func (s *Service) RestoreWithOptions(ctx context.Context, path string, options RestoreOptions) (events.RollbackRecord, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return events.RollbackRecord{}, err
	}
	target, err := s.Store.LatestSnapshot(ctx, absolute)
	if err != nil {
		return events.RollbackRecord{}, err
	}
	run, err := s.Store.GetRun(ctx, target.RunID)
	if err != nil {
		return events.RollbackRecord{}, err
	}
	if !capture.IsWithin(run.WorkspacePath, absolute) {
		return events.RollbackRecord{}, errors.New("restore target is outside recorded workspace")
	}
	if err := validateRestorePath(run.WorkspacePath, absolute); err != nil {
		return events.RollbackRecord{}, err
	}
	if !options.Force {
		if err := checkRestoreConflict(target, absolute); err != nil {
			return events.RollbackRecord{}, err
		}
	}
	actionID := events.NewID("action")
	now := time.Now().UTC()
	safety := events.SnapshotRecord{SnapshotID: events.NewID("snapshot"), RunID: run.RunID, ActionID: actionID, FilePath: absolute, CapturedAt: now}
	if file, openErr := os.Open(absolute); openErr == nil {
		info, statErr := file.Stat()
		if statErr != nil {
			file.Close()
			return events.RollbackRecord{}, statErr
		}
		meta, putErr := s.Snapshots.Put(ctx, snapshots.Metadata{FilePath: absolute}, file)
		file.Close()
		if putErr != nil {
			return events.RollbackRecord{}, putErr
		}
		safety.Exists = true
		safety.ContentHash = meta.ContentHash
		safety.ObjectPath = meta.ObjectPath
		safety.ByteSize = meta.ByteSize
		safety.FileMode = uint32(info.Mode())
	} else if !os.IsNotExist(openErr) {
		return events.RollbackRecord{}, openErr
	}
	expectedAfterRestore := target.Exists
	safety.ExpectedExists = &expectedAfterRestore
	if target.Exists {
		safety.ExpectedHash = target.ContentHash
	}
	if err := s.Store.RecordSnapshot(ctx, safety); err != nil {
		return events.RollbackRecord{}, err
	}
	plan, err := json.Marshal(map[string]any{"path": absolute, "snapshot_id": target.SnapshotID, "force": options.Force})
	if err != nil {
		return events.RollbackRecord{}, err
	}
	record := events.RollbackRecord{RollbackID: events.NewID("rollback"), RunID: run.RunID, RequestedBy: "audit-cli", TargetSnapshotID: target.SnapshotID, Status: "executing", Plan: plan, RequestedAt: now}
	if err := s.Store.CreateRollback(ctx, record); err != nil {
		return events.RollbackRecord{}, err
	}
	var removedBackup string
	if target.Exists {
		if err := s.Snapshots.Restore(ctx, target.ContentHash, absolute, os.FileMode(target.FileMode)); err != nil {
			_ = s.Store.CompleteRollback(ctx, record.RollbackID, "failed", json.RawMessage(`{"restored":false}`), time.Now().UTC())
			return events.RollbackRecord{}, err
		}
	} else {
		removedBackup, err = moveTargetAside(absolute)
		if err != nil {
			_ = s.Store.CompleteRollback(ctx, record.RollbackID, "failed", json.RawMessage(`{"restored":false}`), time.Now().UTC())
			return events.RollbackRecord{}, err
		}
	}
	completed := time.Now().UTC()
	result, err := json.Marshal(map[string]any{"restored": true, "exists": target.Exists, "safety_snapshot_id": safety.SnapshotID})
	if err != nil {
		return record, err
	}
	record.Status, record.Result, record.CompletedAt = "completed", result, &completed
	evidence, err := json.Marshal(record)
	if err != nil {
		return record, err
	}
	if _, err = s.Store.Append(ctx, newEvent(run.RunID, actionID, events.KindRollback, "restore file snapshot", evidence, events.PartlyReversible)); err != nil {
		_ = restoreSafety(ctx, s.Snapshots, safety, absolute, removedBackup)
		_ = s.Store.CompleteRollback(ctx, record.RollbackID, "failed", json.RawMessage(`{"restored":false}`), time.Now().UTC())
		return record, err
	}
	if err := s.Store.CompleteRollback(ctx, record.RollbackID, record.Status, result, completed); err != nil {
		return record, err
	}
	if removedBackup != "" {
		if err := os.Remove(removedBackup); err != nil {
			return record, err
		}
	}
	return record, nil
}

func moveTargetAside(target string) (string, error) {
	if _, err := os.Lstat(target); os.IsNotExist(err) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	placeholder, err := os.CreateTemp(filepath.Dir(target), ".audit-delete-backup-*")
	if err != nil {
		return "", err
	}
	backup := placeholder.Name()
	if err := placeholder.Close(); err != nil {
		return "", err
	}
	if err := os.Remove(backup); err != nil {
		return "", err
	}
	if err := os.Rename(target, backup); err != nil {
		return "", err
	}
	return backup, nil
}

func restoreSafety(ctx context.Context, store *snapshots.DiskStore, safety events.SnapshotRecord, target, removedBackup string) error {
	if removedBackup != "" {
		if _, err := os.Lstat(target); err == nil {
			return errors.New("cannot restore deletion backup because target exists")
		} else if !os.IsNotExist(err) {
			return err
		}
		return os.Rename(removedBackup, target)
	}
	if safety.Exists {
		return store.Restore(ctx, safety.ContentHash, target, os.FileMode(safety.FileMode))
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func checkRestoreConflict(snapshot events.SnapshotRecord, target string) error {
	if snapshot.ExpectedExists == nil {
		return fmt.Errorf("%w: snapshot has no expected current state; use --force for a legacy snapshot", ErrRestoreConflict)
	}
	info, err := os.Lstat(target)
	if !*snapshot.ExpectedExists {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: %s now exists", ErrRestoreConflict, target)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w: %s was deleted after capture", ErrRestoreConflict, target)
		}
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: target is not a regular file", ErrRestoreConflict)
	}
	hash, err := capture.HashFile(target)
	if err != nil {
		return err
	}
	if hash != snapshot.ExpectedHash {
		return fmt.Errorf("%w: current file hash differs from the recorded post-run hash", ErrRestoreConflict)
	}
	return nil
}

func validateRestorePath(workspace, target string) error {
	root, err := filepath.EvalSymlinks(workspace)
	if err != nil {
		return fmt.Errorf("resolve workspace: %w", err)
	}
	parent := filepath.Dir(target)
	for {
		_, statErr := os.Lstat(parent)
		if statErr == nil {
			break
		}
		if !os.IsNotExist(statErr) {
			return statErr
		}
		next := filepath.Dir(parent)
		if next == parent {
			return errors.New("restore target has no existing parent")
		}
		parent = next
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve restore parent: %w", err)
	}
	if !capture.IsWithin(root, resolvedParent) {
		return errors.New("restore target resolves outside recorded workspace")
	}
	if info, err := os.Lstat(target); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return errors.New("restore target is a symbolic link")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) Summary(ctx context.Context, runID string) (map[string]any, error) {
	run, err := s.Store.GetRun(ctx, runID)
	if err != nil {
		return nil, err
	}
	var items []events.Event
	var after uint64
	for {
		page, err := s.Store.Query(ctx, events.Query{RunID: runID, After: after, Limit: 1000})
		if err != nil {
			return nil, err
		}
		items = append(items, page...)
		if len(page) < 1000 {
			break
		}
		after = page[len(page)-1].Sequence
	}
	counts := map[string]int{}
	riskCounts := map[string]int{}
	actions := map[string]struct{}{}
	files := map[string]struct{}{}
	commands, tests, approvals := 0, 0, 0
	for _, item := range items {
		counts[string(item.Kind)]++
		riskCounts[item.Risk.Level]++
		actions[item.ActionID] = struct{}{}
		switch item.Kind {
		case events.KindCommand:
			commands++
			var value struct {
				Command []string `json:"command"`
			}
			if json.Unmarshal(item.Evidence.Data, &value) == nil {
				joined := strings.ToLower(strings.Join(value.Command, " "))
				if strings.Contains(joined, " test") || strings.HasPrefix(joined, "go test") || strings.Contains(joined, "vitest") {
					tests++
				}
			}
		case events.KindFileChange:
			var value struct {
				Path string `json:"path"`
			}
			if json.Unmarshal(item.Evidence.Data, &value) == nil && value.Path != "" {
				files[value.Path] = struct{}{}
			}
		case events.KindApproval:
			approvals++
		}
	}
	duration := time.Since(run.StartedAt)
	if run.EndedAt != nil {
		duration = run.EndedAt.Sub(run.StartedAt)
	}
	rollbackAvailability, rollbackErr := s.Store.RollbackAvailability(ctx, runID)
	if rollbackErr != nil {
		rollbackAvailability = "unknown"
	}
	integrityStatus := "verified"
	if err := s.Store.VerifyRun(ctx, runID); err != nil {
		integrityStatus = "broken"
	}
	return map[string]any{"run": run, "status": run.Status, "agent": map[string]string{"type": run.AgentType, "id": run.AgentID}, "duration_ms": duration.Milliseconds(), "action_count": len(actions), "event_count": len(items), "events_by_kind": counts, "files_changed": len(files), "commands_executed": commands, "tests": tests, "risk_counts": riskCounts, "approvals": approvals, "rollback_availability": rollbackAvailability, "integrity_status": integrityStatus, "integrity_valid": integrityStatus == "verified"}, nil
}
func (s *Service) StringResult(result RunResult) string {
	return fmt.Sprintf("run %s: %s, exit=%d, file_changes=%d", result.Run.RunID, result.Run.Status, result.ExitCode, result.Changes)
}

type captureBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func newCaptureBuffer(limit int) *captureBuffer { return &captureBuffer{limit: limit} }

func (b *captureBuffer) Write(data []byte) (int, error) {
	original := len(data)
	remaining := b.limit - b.buffer.Len()
	if remaining > 0 {
		if len(data) > remaining {
			data = data[:remaining]
		}
		_, _ = b.buffer.Write(data)
	}
	if original > len(data) || remaining <= 0 {
		b.truncated = true
	}
	return original, nil
}

func (b *captureBuffer) String() string {
	if b.truncated {
		return b.buffer.String() + "\n[OUTPUT TRUNCATED BY AGENT AUDIT CONSOLE]"
	}
	return b.buffer.String()
}

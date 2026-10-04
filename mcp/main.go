package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/adapters"
	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/buildinfo"
	runtimeconfig "github.com/agent-audit-console/agent-audit-console/internal/config"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/agent-audit-console/agent-audit-console/internal/policy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const protocolVersion = "2.0"
const maxPage = 1000
const defaultPage = 100
const maxText = 64 << 10

type errorCode string

const (
	invalidArgument  errorCode = "INVALID_ARGUMENT"
	notFound         errorCode = "NOT_FOUND"
	conflict         errorCode = "CONFLICT"
	policyDenied     errorCode = "POLICY_DENIED"
	approvalRequired errorCode = "APPROVAL_REQUIRED"
	unauthorized     errorCode = "UNAUTHORIZED"
	integrityError   errorCode = "INTEGRITY_ERROR"
	internalError    errorCode = "INTERNAL"
)

type toolError struct {
	Code    errorCode
	Message string
}

func (e *toolError) Error() string              { return string(e.Code) + ": " + e.Message }
func fail(code errorCode, message string) error { return &toolError{code, message} }
func safeError(err error) error {
	if err == nil {
		return nil
	}
	var coded *toolError
	if errors.As(err, &coded) {
		return err
	}
	if errors.Is(err, events.ErrNotFound) {
		return fail(notFound, "resource not found")
	}
	var integrity *events.IntegrityError
	if errors.As(err, &integrity) {
		return fail(integrityError, "run integrity verification failed")
	}
	return fail(internalError, "operation failed")
}

type runRef struct {
	RunID         string `json:"run_id,omitempty"`
	ContextKey    string `json:"context_key,omitempty"`
	AgentType     string `json:"agent_type,omitempty"`
	SessionID     string `json:"session_id,omitempty"`
	WorkspacePath string `json:"workspace_path,omitempty"`
}
type startRunInput struct {
	runRef
	AgentID         string `json:"agent_id,omitempty"`
	AgentName       string `json:"agent_name,omitempty"`
	AgentVersion    string `json:"agent_version,omitempty"`
	Repository      string `json:"repository,omitempty"`
	RepositoryRoot  string `json:"repository_root,omitempty"`
	GitBranch       string `json:"git_branch,omitempty"`
	GitCommitBefore string `json:"git_commit_before,omitempty"`
	TriggerSource   string `json:"trigger_source,omitempty"`
	IdempotencyKey  string `json:"idempotency_key,omitempty"`
}
type startRunOutput struct {
	RunID      string `json:"run_id"`
	Status     string `json:"status"`
	ContextKey string `json:"context_key"`
	Replayed   bool   `json:"replayed,omitempty"`
}
type finishRunInput struct {
	runRef
	Status         string `json:"status"`
	GitCommitAfter string `json:"git_commit_after,omitempty"`
}
type runOutput struct {
	Run events.Run `json:"run"`
}
type actionInput struct {
	runRef
	ActionID       string         `json:"action_id,omitempty"`
	ParentActionID string         `json:"parent_action_id,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
	Kind           events.Kind    `json:"kind"`
	Intent         string         `json:"intent"`
	Evidence       map[string]any `json:"evidence,omitempty"`
	ActorID        string         `json:"actor_id,omitempty"`
	Reversibility  string         `json:"reversibility,omitempty"`
	IdempotencyKey string         `json:"idempotency_key,omitempty"`
}
type actionOutput struct {
	EventID       string              `json:"event_id"`
	ActionID      string              `json:"action_id"`
	Sequence      uint64              `json:"sequence"`
	IntegrityHash string              `json:"integrity_hash"`
	Risk          string              `json:"risk"`
	PolicyStatus  string              `json:"policy_status"`
	Status        events.ActionStatus `json:"status"`
	Replayed      bool                `json:"replayed,omitempty"`
}
type pageInput struct {
	runRef
	Limit   int    `json:"limit,omitempty"`
	Cursor  string `json:"cursor,omitempty"`
	Compact bool   `json:"compact,omitempty"`
}
type eventInput struct {
	EventID string `json:"event_id"`
	Compact bool   `json:"compact,omitempty"`
}
type eventsOutput struct {
	Events    []events.Event `json:"events"`
	HasMore   bool           `json:"has_more"`
	Cursor    string         `json:"cursor,omitempty"`
	Truncated bool           `json:"truncated,omitempty"`
}
type runsOutput struct {
	Runs    []events.Run `json:"runs"`
	HasMore bool         `json:"has_more"`
	Cursor  string       `json:"cursor,omitempty"`
}
type summaryOutput struct {
	Summary   map[string]any `json:"summary"`
	Truncated bool           `json:"truncated,omitempty"`
}
type policyInput struct {
	Kind        events.Kind `json:"kind"`
	Intent      string      `json:"intent"`
	Command     []string    `json:"command,omitempty"`
	Path        string      `json:"path,omitempty"`
	Target      string      `json:"target,omitempty"`
	Agent       string      `json:"agent,omitempty"`
	Workspace   string      `json:"workspace,omitempty"`
	Environment string      `json:"environment,omitempty"`
}
type policyOutput struct {
	Risk              string             `json:"risk"`
	Decision          string             `json:"decision"`
	CanonicalDecision string             `json:"canonical_decision"`
	MatchedRule       string             `json:"matched_rule,omitempty"`
	Reason            string             `json:"reason,omitempty"`
	Source            string             `json:"source,omitempty"`
	MatchedRules      []policy.RuleMatch `json:"matched_rules"`
	Trace             []policy.TraceStep `json:"trace"`
	Fingerprint       string             `json:"policy_fingerprint"`
	RequiresApproval  bool               `json:"requires_approval"`
}
type approvalInput struct {
	actionInput
	Reason string `json:"reason,omitempty"`
}
type approvalStatusInput struct {
	runRef
	ActionID string `json:"action_id"`
}
type approvalOutput struct {
	ActionID string `json:"action_id"`
	Status   string `json:"status"`
	Message  string `json:"message,omitempty"`
}
type rollbackInput struct {
	runRef
	TargetEventID    string `json:"target_event_id,omitempty"`
	TargetSnapshotID string `json:"target_snapshot_id,omitempty"`
	Reason           string `json:"reason,omitempty"`
	IdempotencyKey   string `json:"idempotency_key,omitempty"`
}
type rollbackStatusInput struct {
	RollbackID string `json:"rollback_id"`
}
type rollbackOutput struct {
	RollbackID string `json:"rollback_id"`
	Status     string `json:"status"`
	Replayed   bool   `json:"replayed,omitempty"`
}
type rollbackPreview struct {
	RunID         string           `json:"run_id"`
	AffectedFiles []map[string]any `json:"affected_files"`
	Conflicts     []string         `json:"conflicts"`
	Warnings      []string         `json:"warnings"`
	Reversibility string           `json:"reversibility"`
}
type diffInput struct {
	runRef
	Path    string `json:"path"`
	Compact bool   `json:"compact,omitempty"`
}
type diffOutput struct {
	Path          string `json:"path"`
	BeforeHash    string `json:"before_hash,omitempty"`
	AfterHash     string `json:"after_hash,omitempty"`
	Diff          string `json:"diff,omitempty"`
	Truncated     bool   `json:"truncated"`
	Reversibility string `json:"reversibility"`
}

func main() {
	dataDir := flag.String("data-dir", "", "audit data directory")
	configFile := flag.String("config", "", "YAML config file")
	flag.Parse()
	resolved, err := runtimeconfig.Load(*configFile)
	if err != nil {
		log.Fatal(err)
	}
	if *dataDir != "" {
		resolved, err = resolved.WithDataDir(*dataDir)
		if err != nil {
			log.Fatal(err)
		}
	}
	service, err := audit.OpenWithConfig(resolved)
	if err != nil {
		log.Fatal(err)
	}
	defer service.Close()
	if err := newServer(service).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func newServer(service *audit.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-audit-console", Version: buildinfo.Version}, nil)
	tools := []string{"start_run", "get_run", "get_run_summary", "finish_run", "plan_action", "record_action", "complete_action", "fail_action", "list_runs", "list_events", "get_event", "get_file_diff", "evaluate_action", "simulate_policy", "explain_policy", "request_approval", "get_approval_status", "preview_rollback", "request_rollback", "get_rollback_status", "health", "capabilities"}
	mcp.AddTool(server, &mcp.Tool{Name: "health", Description: "Check local MCP dependencies without exposing secrets"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, map[string]any, error) {
		status := "ok"
		database, snapshot, policy := "ok", "ok", "ok"
		if err := service.Store.Ready(ctx); err != nil {
			database = "error"
			status = "degraded"
		}
		if err := service.Snapshots.Ready(ctx); err != nil {
			snapshot = "error"
			status = "degraded"
		}
		if err := service.Policy.Ready(); err != nil {
			policy = "error"
			status = "degraded"
		}
		return nil, map[string]any{"status": status, "version": buildinfo.Version, "database": database, "snapshot_store": snapshot, "policy": policy, "server_time": time.Now().UTC()}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "capabilities", Description: "Discover MCP, adapter, event, policy, approval and rollback capabilities"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, map[string]any, error) {
		return nil, map[string]any{"server_version": buildinfo.Version, "protocol_version": protocolVersion, "audit_capabilities_version": "2", "tools": tools, "adapters": service.Adapters.List(), "event_kinds": []events.Kind{events.KindCommand, events.KindFileChange, events.KindGit, events.KindMCPCall, events.KindApproval, events.KindRollback}, "policy_decisions": []string{policy.DecisionAllow, policy.DecisionWarn, policy.DecisionApproval, policy.DecisionDeny}, "error_codes": []errorCode{invalidArgument, notFound, conflict, policyDenied, approvalRequired, unauthorized, integrityError, internalError}, "approval_support": true, "rollback_support": true, "policy_simulation": true, "policy_validation": true, "policy_trace": true, "policy_test_support": true}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "start_run", Description: "Start a run and optionally bind a safe automatic context"}, func(ctx context.Context, _ *mcp.CallToolRequest, input startRunInput) (*mcp.CallToolResult, startRunOutput, error) {
		out, err := startRun(ctx, service, input)
		return nil, out, safeError(err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_run", Description: "Get a run by explicit ID or context"}, func(ctx context.Context, _ *mcp.CallToolRequest, input runRef) (*mcp.CallToolResult, runOutput, error) {
		id, err := resolveRun(ctx, service, input)
		if err != nil {
			return nil, runOutput{}, safeError(err)
		}
		run, err := service.Store.GetRun(ctx, id)
		return nil, runOutput{run}, safeError(err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "finish_run", Description: "Finish a running audit run"}, func(ctx context.Context, _ *mcp.CallToolRequest, input finishRunInput) (*mcp.CallToolResult, runOutput, error) {
		id, err := resolveRun(ctx, service, input.runRef)
		if err != nil {
			return nil, runOutput{}, safeError(err)
		}
		status := input.Status
		if status == "" {
			status = string(events.RunStatusCompleted)
		}
		if status != string(events.RunStatusCompleted) && status != string(events.RunStatusFailed) && status != string(events.RunStatusCancelled) {
			return nil, runOutput{}, fail(invalidArgument, "status must be completed, failed, or cancelled")
		}
		if err = service.Store.FinishRun(ctx, id, status, time.Now().UTC()); err != nil {
			return nil, runOutput{}, safeError(err)
		}
		if err = service.Store.MergeRunMetadata(ctx, id, map[string]string{"git_commit_after": input.GitCommitAfter}); err != nil {
			return nil, runOutput{}, safeError(err)
		}
		run, err := service.Store.GetRun(ctx, id)
		return nil, runOutput{run}, safeError(err)
	})
	addActionTools(server, service)
	mcp.AddTool(server, &mcp.Tool{Name: "list_runs", Description: "List runs with bounded cursor pagination"}, func(ctx context.Context, _ *mcp.CallToolRequest, input pageInput) (*mcp.CallToolResult, runsOutput, error) {
		limit := bounded(input.Limit)
		items, err := service.Store.ListRunsPage(ctx, input.Cursor, limit+1)
		if err != nil {
			return nil, runsOutput{}, safeError(err)
		}
		more := len(items) > limit
		if more {
			items = items[:limit]
		}
		cursor := ""
		if more && len(items) > 0 {
			last := items[len(items)-1]
			cursor = last.StartedAt.UTC().Format(time.RFC3339Nano) + "|" + last.RunID
		}
		return nil, runsOutput{items, more, cursor}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_events", Description: "List run events with bounded cursor pagination and compact mode"}, func(ctx context.Context, _ *mcp.CallToolRequest, input pageInput) (*mcp.CallToolResult, eventsOutput, error) {
		id, err := resolveRun(ctx, service, input.runRef)
		if err != nil {
			return nil, eventsOutput{}, safeError(err)
		}
		after, _ := strconv.ParseUint(input.Cursor, 10, 64)
		limit := bounded(input.Limit)
		items, err := service.Store.Query(ctx, events.Query{RunID: id, After: after, Limit: limit + 1})
		if err != nil {
			return nil, eventsOutput{}, safeError(err)
		}
		more := len(items) > limit
		if more {
			items = items[:limit]
		}
		truncated := false
		if input.Compact {
			for i := range items {
				if len(items[i].Evidence.Data) > 1024 {
					items[i].Evidence.Data = json.RawMessage(`{"truncated":true}`)
					truncated = true
				}
			}
		}
		cursor := ""
		if more && len(items) > 0 {
			cursor = strconv.FormatUint(items[len(items)-1].Sequence, 10)
		}
		return nil, eventsOutput{items, more, cursor, truncated}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_event", Description: "Get one event"}, func(ctx context.Context, _ *mcp.CallToolRequest, input eventInput) (*mcp.CallToolResult, events.Event, error) {
		if input.EventID == "" {
			return nil, events.Event{}, fail(invalidArgument, "event_id is required")
		}
		item, err := service.Store.ByID(ctx, input.EventID)
		if err == nil && input.Compact && len(item.Evidence.Data) > 1024 {
			item.Evidence.Data = json.RawMessage(`{"truncated":true}`)
		}
		return nil, item, safeError(err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_run_summary", Description: "Get a compact run and action summary"}, func(ctx context.Context, _ *mcp.CallToolRequest, input pageInput) (*mcp.CallToolResult, summaryOutput, error) {
		id, err := resolveRun(ctx, service, input.runRef)
		if err != nil {
			return nil, summaryOutput{}, safeError(err)
		}
		summary, err := service.Summary(ctx, id)
		return nil, summaryOutput{summary, false}, safeError(err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_file_diff", Description: "Get a bounded file diff for a run"}, func(ctx context.Context, _ *mcp.CallToolRequest, input diffInput) (*mcp.CallToolResult, diffOutput, error) {
		out, err := fileDiff(ctx, service, input)
		return nil, out, safeError(err)
	})
	addPolicyApprovalTools(server, service)
	addRollbackTools(server, service)
	return server
}

func startRun(ctx context.Context, s *audit.Service, input startRunInput) (startRunOutput, error) {
	if input.IdempotencyKey != "" {
		var out startRunOutput
		if ok, err := s.Store.GetIdempotency(ctx, "start_run", input.IdempotencyKey, &out); err != nil {
			return out, err
		} else if ok {
			out.Replayed = true
			return out, nil
		}
	}
	workspace := input.WorkspacePath
	if workspace == "" {
		workspace = "."
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return startRunOutput{}, fail(invalidArgument, "invalid workspace_path")
	}
	agent := input.AgentType
	if agent == "" {
		agent = "custom"
	}
	adapter, err := s.Adapters.Resolve(agent)
	if err != nil {
		return startRunOutput{}, fail(invalidArgument, "unknown agent_type")
	}
	if input.AgentID == "" {
		input.AgentID = "mcp-agent"
	}
	metadata := map[string]string{"agent_name": input.AgentName, "agent_version": input.AgentVersion, "repository": input.Repository, "repository_root": input.RepositoryRoot, "git_branch": input.GitBranch, "git_commit_before": input.GitCommitBefore, "session_id": input.SessionID, "trigger_source": input.TriggerSource}
	run := events.Run{RunID: events.NewID("run"), AgentType: adapter.Descriptor().Name, AgentID: input.AgentID, Status: string(events.RunStatusRunning), WorkspacePath: absolute, StartedAt: time.Now().UTC(), Metadata: metadata}
	if err = s.Store.CreateRun(ctx, run); err != nil {
		return startRunOutput{}, err
	}
	if input.ContextKey == "" && input.SessionID == "" {
		input.ContextKey = events.NewID("context")
	}
	key := contextKey(input.runRef, absolute, agent)
	if err = s.Store.BindRunContext(ctx, key, run.RunID); err != nil {
		return startRunOutput{}, err
	}
	out := startRunOutput{run.RunID, run.Status, key, false}
	if err = s.Store.PutIdempotency(ctx, "start_run", input.IdempotencyKey, out); err != nil {
		return startRunOutput{}, err
	}
	return out, nil
}
func contextKey(ref runRef, workspace, agent string) string {
	if ref.ContextKey != "" {
		if strings.HasPrefix(ref.ContextKey, "explicit:") || strings.HasPrefix(ref.ContextKey, "auto:") {
			return ref.ContextKey
		}
		sum := sha256.Sum256([]byte(ref.ContextKey))
		return "explicit:" + hex.EncodeToString(sum[:16])
	}
	raw := strings.Join([]string{agent, ref.SessionID, filepath.Clean(workspace), strconv.Itoa(os.Getpid())}, "\x00")
	sum := sha256.Sum256([]byte(raw))
	return "auto:" + hex.EncodeToString(sum[:16])
}
func resolveRun(ctx context.Context, s *audit.Service, ref runRef) (string, error) {
	if ref.RunID != "" {
		return ref.RunID, nil
	}
	workspace := ref.WorkspacePath
	if workspace == "" {
		workspace = "."
	}
	absolute, err := filepath.Abs(workspace)
	if err != nil {
		return "", fail(invalidArgument, "invalid workspace")
	}
	agent := ref.AgentType
	if agent == "" {
		agent = "custom"
	}
	id, err := s.Store.ResolveRunContext(ctx, contextKey(ref, absolute, agent))
	if err != nil {
		return "", fail(notFound, "no running run for context")
	}
	return id, nil
}
func bounded(value int) int {
	if value <= 0 {
		return defaultPage
	}
	if value > maxPage {
		return maxPage
	}
	return value
}

func addActionTools(server *mcp.Server, s *audit.Service) {
	register := func(name string, status events.ActionStatus, description string) {
		mcp.AddTool(server, &mcp.Tool{Name: name, Description: description}, func(ctx context.Context, _ *mcp.CallToolRequest, input actionInput) (*mcp.CallToolResult, actionOutput, error) {
			out, err := writeAction(ctx, s, input, status)
			return nil, out, safeError(err)
		})
	}
	register("plan_action", events.ActionPlanned, "Plan an action and evaluate policy")
	register("record_action", events.ActionStarted, "Record or start an action (v0.1 compatible)")
	register("complete_action", events.ActionCompleted, "Complete an action")
	register("fail_action", events.ActionFailed, "Fail an action")
}
func writeAction(ctx context.Context, s *audit.Service, input actionInput, status events.ActionStatus) (actionOutput, error) {
	id, err := resolveRun(ctx, s, input.runRef)
	if err != nil {
		return actionOutput{}, err
	}
	scope := string(status) + ":" + id
	if input.IdempotencyKey != "" {
		var out actionOutput
		if ok, err := s.Store.GetIdempotency(ctx, scope, input.IdempotencyKey, &out); err != nil {
			return out, err
		} else if ok {
			out.Replayed = true
			return out, nil
		}
	}
	if input.ActionID == "" {
		input.ActionID = events.NewID("action")
	}
	if input.Kind == "" {
		input.Kind = events.KindMCPCall
	}
	if input.Intent == "" {
		return actionOutput{}, fail(invalidArgument, "intent is required")
	}
	if input.ActorID == "" {
		input.ActorID = "mcp-agent"
	}
	if input.Reversibility == "" {
		input.Reversibility = events.Irreversible
	}
	run, err := s.Store.GetRun(ctx, id)
	if err != nil {
		return actionOutput{}, err
	}
	if run.Status != string(events.RunStatusRunning) {
		return actionOutput{}, fail(conflict, "run is not running")
	}
	adapter, err := s.Adapters.Resolve(run.AgentType)
	if err != nil {
		return actionOutput{}, err
	}
	evidence, err := json.Marshal(s.Redactor.Value(input.Evidence))
	if err != nil {
		return actionOutput{}, fail(invalidArgument, "evidence is not valid JSON")
	}
	if len(evidence) > 1<<20 || len(input.Intent) > 16<<10 {
		return actionOutput{}, fail(invalidArgument, "action payload exceeds size limit")
	}
	event, err := adapter.Normalize(adapters.Action{RunID: id, ActionID: input.ActionID, ParentActionID: input.ParentActionID, CorrelationID: input.CorrelationID, Status: status, ActorID: input.ActorID, Kind: input.Kind, Intent: s.Redactor.String(input.Intent), Evidence: events.Evidence{Data: evidence}, Reversibility: events.Reversibility{Status: input.Reversibility}})
	if err != nil {
		return actionOutput{}, fail(invalidArgument, err.Error())
	}
	risk, decision, err := s.Policy.Evaluate(ctx, event)
	if err != nil {
		return actionOutput{}, err
	}
	event.Risk, event.PolicyDecision = risk, decision
	if decision.Status == events.PolicyRejected || decision.Status == events.PolicyPending {
		event.ActionStatus = events.ActionBlocked
	}
	sealed, err := s.Store.Append(ctx, event)
	if err != nil {
		return actionOutput{}, err
	}
	if err = s.Store.UpsertAction(ctx, events.ActionRecord{RunID: id, ActionID: input.ActionID, ParentActionID: input.ParentActionID, CorrelationID: input.CorrelationID, Kind: string(input.Kind), Intent: event.Intent, Status: event.ActionStatus}); err != nil {
		return actionOutput{}, err
	}
	out := actionOutput{sealed.EventID, sealed.ActionID, sealed.Sequence, sealed.IntegrityHash, risk.Level, decision.Status, sealed.ActionStatus, false}
	if err = s.Store.PutIdempotency(ctx, scope, input.IdempotencyKey, out); err != nil {
		return actionOutput{}, err
	}
	return out, nil
}

func addPolicyApprovalTools(server *mcp.Server, s *audit.Service) {
	handler := func(ctx context.Context, _ *mcp.CallToolRequest, input policyInput) (*mcp.CallToolResult, policyOutput, error) {
		if input.Kind == "" {
			input.Kind = events.KindCommand
		}
		raw, _ := json.Marshal(map[string]any{"command": input.Command, "path": input.Path, "target": input.Target, "agent": input.Agent, "workspace": input.Workspace, "environment": input.Environment})
		event := events.Event{SchemaVersion: 2, EventID: "evaluation", RunID: "evaluation", ActionID: "evaluation", Timestamp: time.Now().UTC(), Sequence: 1, Actor: events.Actor{Type: "agent", ID: "mcp-agent"}, Kind: input.Kind, Intent: input.Intent, Evidence: events.Evidence{Data: raw}, Risk: events.Risk{Level: events.RiskUnknown}, PolicyDecision: events.PolicyDecision{Status: events.PolicyNotEvaluated}, Reversibility: events.Reversibility{Status: events.NotApplicable}}
		result, err := s.Policy.Simulate(ctx, event)
		if err != nil {
			return nil, policyOutput{}, safeError(err)
		}
		return nil, policyOutput{Risk: result.RiskLevel, Decision: policy.EventDecision(result.Decision), CanonicalDecision: result.Decision, MatchedRule: result.RuleID, Reason: result.Reason, Source: result.PolicySource, MatchedRules: result.MatchedRules, Trace: result.Trace, Fingerprint: result.PolicyFingerprint, RequiresApproval: result.RequiresApproval}, nil
	}
	mcp.AddTool(server, &mcp.Tool{Name: "evaluate_action", Description: "Evaluate an action without recording it"}, handler)
	mcp.AddTool(server, &mcp.Tool{Name: "simulate_policy", Description: "Simulate policy preflight without creating audit or approval state"}, handler)
	mcp.AddTool(server, &mcp.Tool{Name: "explain_policy", Description: "Explain the matching policy decision without exposing paths"}, handler)
	mcp.AddTool(server, &mcp.Tool{Name: "request_approval", Description: "Request approval; this tool cannot approve its own request"}, func(ctx context.Context, _ *mcp.CallToolRequest, input approvalInput) (*mcp.CallToolResult, approvalOutput, error) {
		out, err := requestApproval(ctx, s, input)
		if err != nil {
			return nil, approvalOutput{}, safeError(err)
		}
		return nil, out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_approval_status", Description: "Read approval status for an action"}, func(ctx context.Context, _ *mcp.CallToolRequest, input approvalStatusInput) (*mcp.CallToolResult, approvalOutput, error) {
		id, err := resolveRun(ctx, s, input.runRef)
		if err != nil {
			return nil, approvalOutput{}, safeError(err)
		}
		items, err := s.Store.Query(ctx, events.Query{RunID: id, Kinds: []events.Kind{events.KindApproval}, Limit: maxPage})
		if err != nil {
			return nil, approvalOutput{}, safeError(err)
		}
		status := "not_requested"
		for _, item := range items {
			if item.ActionID == input.ActionID {
				status = item.PolicyDecision.Status
			}
		}
		return nil, approvalOutput{input.ActionID, status, ""}, nil
	})
}

func requestApproval(ctx context.Context, s *audit.Service, input approvalInput) (approvalOutput, error) {
	id, err := resolveRun(ctx, s, input.runRef)
	if err != nil {
		return approvalOutput{}, err
	}
	if input.ActionID == "" {
		input.ActionID = events.NewID("action")
	}
	run, err := s.Store.GetRun(ctx, id)
	if err != nil {
		return approvalOutput{}, err
	}
	adapter, err := s.Adapters.Resolve(run.AgentType)
	if err != nil {
		return approvalOutput{}, err
	}
	evidence, _ := json.Marshal(map[string]any{"reason": s.Redactor.String(input.Reason), "requested_by": "mcp-agent"})
	event, err := adapter.Normalize(adapters.Action{RunID: id, ActionID: input.ActionID, ParentActionID: input.ParentActionID, CorrelationID: input.CorrelationID, Status: events.ActionBlocked, ActorID: "mcp-agent", Kind: events.KindApproval, Intent: "request approval", Evidence: events.Evidence{Data: evidence}, Reversibility: events.Reversibility{Status: events.NotApplicable}})
	if err != nil {
		return approvalOutput{}, err
	}
	event.Risk = events.Risk{Level: events.RiskHigh, Reasons: []string{"explicit approval requested"}}
	event.PolicyDecision = events.PolicyDecision{Status: events.PolicyPending, Reason: "awaiting authorized user decision"}
	if _, err = s.Store.Append(ctx, event); err != nil {
		return approvalOutput{}, err
	}
	if err = s.Store.UpsertAction(ctx, events.ActionRecord{RunID: id, ActionID: input.ActionID, ParentActionID: input.ParentActionID, CorrelationID: input.CorrelationID, Kind: string(events.KindApproval), Intent: event.Intent, Status: events.ActionBlocked}); err != nil {
		return approvalOutput{}, err
	}
	return approvalOutput{input.ActionID, "pending", "Approval must be completed by an authorized user or UI; MCP cannot self-approve."}, nil
}

func addRollbackTools(server *mcp.Server, s *audit.Service) {
	mcp.AddTool(server, &mcp.Tool{Name: "preview_rollback", Description: "Preview rollback effects without modifying files"}, func(ctx context.Context, _ *mcp.CallToolRequest, input rollbackInput) (*mcp.CallToolResult, rollbackPreview, error) {
		out, err := previewRollback(ctx, s, input)
		return nil, out, safeError(err)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "request_rollback", Description: "Create a pending rollback request; does not execute restore"}, func(ctx context.Context, _ *mcp.CallToolRequest, input rollbackInput) (*mcp.CallToolResult, rollbackOutput, error) {
		id, err := resolveRun(ctx, s, input.runRef)
		if err != nil {
			return nil, rollbackOutput{}, safeError(err)
		}
		scope := "request_rollback:" + id
		if input.IdempotencyKey != "" {
			var out rollbackOutput
			if ok, err := s.Store.GetIdempotency(ctx, scope, input.IdempotencyKey, &out); err != nil {
				return nil, out, safeError(err)
			} else if ok {
				out.Replayed = true
				return nil, out, nil
			}
		}
		if _, err = previewRollback(ctx, s, input); err != nil {
			return nil, rollbackOutput{}, safeError(err)
		}
		plan, _ := json.Marshal(map[string]any{"reason": s.Redactor.String(input.Reason)})
		record := events.RollbackRecord{RollbackID: events.NewID("rollback"), RunID: id, RequestedBy: "mcp-agent", TargetEventID: input.TargetEventID, TargetSnapshotID: input.TargetSnapshotID, Status: "pending", Plan: plan, RequestedAt: time.Now().UTC()}
		if err = s.Store.CreateRollback(ctx, record); err != nil {
			return nil, rollbackOutput{}, safeError(err)
		}
		out := rollbackOutput{record.RollbackID, record.Status, false}
		if err = s.Store.PutIdempotency(ctx, scope, input.IdempotencyKey, out); err != nil {
			return nil, rollbackOutput{}, safeError(err)
		}
		return nil, out, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_rollback_status", Description: "Read rollback request status"}, func(ctx context.Context, _ *mcp.CallToolRequest, input rollbackStatusInput) (*mcp.CallToolResult, rollbackOutput, error) {
		record, err := s.Store.RollbackByID(ctx, input.RollbackID)
		return nil, rollbackOutput{record.RollbackID, record.Status, false}, safeError(err)
	})
}
func previewRollback(ctx context.Context, s *audit.Service, input rollbackInput) (rollbackPreview, error) {
	id, err := resolveRun(ctx, s, input.runRef)
	if err != nil {
		return rollbackPreview{}, err
	}
	if input.TargetEventID == "" && input.TargetSnapshotID == "" {
		return rollbackPreview{}, fail(invalidArgument, "target_event_id or target_snapshot_id is required")
	}
	out := rollbackPreview{RunID: id, Reversibility: "unknown", AffectedFiles: []map[string]any{}, Conflicts: []string{}, Warnings: []string{}}
	if input.TargetEventID != "" {
		item, err := s.Store.ByID(ctx, input.TargetEventID)
		if err != nil || item.RunID != id {
			return out, fail(invalidArgument, "target event does not belong to run")
		}
		out.Reversibility = item.Reversibility.Status
	}
	if input.TargetSnapshotID != "" {
		snap, err := s.Store.SnapshotByID(ctx, input.TargetSnapshotID)
		if err != nil || snap.RunID != id {
			return out, fail(invalidArgument, "target snapshot does not belong to run")
		}
		currentHash := ""
		if data, readErr := os.ReadFile(snap.FilePath); readErr == nil {
			sum := sha256.Sum256(data)
			currentHash = hex.EncodeToString(sum[:])
		}
		conflictValue := snap.ExpectedExists != nil && *snap.ExpectedExists && currentHash != "" && currentHash != snap.ExpectedHash
		if conflictValue {
			out.Conflicts = append(out.Conflicts, "current file hash differs from recorded post-run hash")
		}
		out.AffectedFiles = append(out.AffectedFiles, map[string]any{"path": snap.FilePath, "current_hash": currentHash, "target_hash": snap.ContentHash, "conflict": conflictValue})
		out.Reversibility = "reversible"
	}
	return out, nil
}
func fileDiff(ctx context.Context, s *audit.Service, input diffInput) (diffOutput, error) {
	id, err := resolveRun(ctx, s, input.runRef)
	if err != nil {
		return diffOutput{}, err
	}
	run, err := s.Store.GetRun(ctx, id)
	if err != nil {
		return diffOutput{}, err
	}
	path := input.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(run.WorkspacePath, path)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return diffOutput{}, fail(invalidArgument, "invalid path")
	}
	relative, err := filepath.Rel(run.WorkspacePath, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return diffOutput{}, fail(unauthorized, "path is outside the run workspace")
	}
	var found diffOutput
	var after uint64
	foundMatch := false
	for {
		items, queryErr := s.Store.Query(ctx, events.Query{RunID: id, Kinds: []events.Kind{events.KindFileChange}, After: after, Limit: maxPage})
		if queryErr != nil {
			return diffOutput{}, queryErr
		}
		for i := range items {
			var value struct {
				Path       string `json:"path"`
				BeforeHash string `json:"before_hash"`
				AfterHash  string `json:"after_hash"`
				Diff       string `json:"diff"`
			}
			if json.Unmarshal(items[i].Evidence.Data, &value) == nil && filepath.Clean(value.Path) == filepath.Clean(path) {
				limit := maxText
				if input.Compact {
					limit = 4096
				}
				diff, truncated := truncate(value.Diff, limit)
				found = diffOutput{value.Path, value.BeforeHash, value.AfterHash, diff, truncated, items[i].Reversibility.Status}
				foundMatch = true
			}
		}
		if len(items) < maxPage {
			break
		}
		after = items[len(items)-1].Sequence
	}
	if foundMatch {
		return found, nil
	}
	return diffOutput{}, events.ErrNotFound
}
func truncate(value string, limit int) (string, bool) {
	if len(value) <= limit {
		return value, false
	}
	return value[:limit], true
}

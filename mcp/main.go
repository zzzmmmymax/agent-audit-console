package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/adapters"
	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/buildinfo"
	runtimeconfig "github.com/agent-audit-console/agent-audit-console/internal/config"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type startRunInput struct {
	AgentType     string `json:"agent_type"`
	AgentID       string `json:"agent_id"`
	WorkspacePath string `json:"workspace_path"`
}
type startRunOutput struct {
	RunID  string `json:"run_id"`
	Status string `json:"status"`
}
type recordActionInput struct {
	RunID         string         `json:"run_id"`
	ActionID      string         `json:"action_id,omitempty"`
	Kind          events.Kind    `json:"kind"`
	Intent        string         `json:"intent"`
	Evidence      map[string]any `json:"evidence,omitempty"`
	ActorID       string         `json:"actor_id,omitempty"`
	Reversibility string         `json:"reversibility,omitempty"`
}
type recordActionOutput struct {
	EventID       string `json:"event_id"`
	Sequence      uint64 `json:"sequence"`
	IntegrityHash string `json:"integrity_hash"`
	Risk          string `json:"risk"`
	PolicyStatus  string `json:"policy_status"`
}
type runInput struct {
	RunID string `json:"run_id"`
}
type summaryOutput struct {
	Summary map[string]any `json:"summary"`
}
type rollbackInput struct {
	RunID            string `json:"run_id"`
	TargetEventID    string `json:"target_event_id,omitempty"`
	TargetSnapshotID string `json:"target_snapshot_id,omitempty"`
	Reason           string `json:"reason,omitempty"`
}
type rollbackOutput struct {
	RollbackID string `json:"rollback_id"`
	Status     string `json:"status"`
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
	server := newServer(service)
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}

func newServer(service *audit.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-audit-console", Version: buildinfo.Version}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "start_run", Description: "Start a local agent audit run"}, func(ctx context.Context, _ *mcp.CallToolRequest, input startRunInput) (*mcp.CallToolResult, startRunOutput, error) {
		workspace, err := filepath.Abs(input.WorkspacePath)
		if err != nil {
			return nil, startRunOutput{}, err
		}
		if input.AgentType == "" {
			input.AgentType = "custom"
		}
		if input.AgentID == "" {
			input.AgentID = "unknown-agent"
		}
		agentAdapter, err := service.Adapters.Resolve(input.AgentType)
		if err != nil {
			return nil, startRunOutput{}, err
		}
		run := events.Run{RunID: events.NewID("run"), AgentType: agentAdapter.Descriptor().Name, AgentID: input.AgentID, Status: "running", WorkspacePath: workspace, StartedAt: time.Now().UTC(), Metadata: map[string]string{"source": "mcp"}}
		if err := service.Store.CreateRun(ctx, run); err != nil {
			return nil, startRunOutput{}, err
		}
		return nil, startRunOutput{RunID: run.RunID, Status: run.Status}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "record_action", Description: "Append an action to a run's tamper-evident event chain"}, func(ctx context.Context, _ *mcp.CallToolRequest, input recordActionInput) (*mcp.CallToolResult, recordActionOutput, error) {
		if input.ActionID == "" {
			input.ActionID = events.NewID("action")
		}
		if input.ActorID == "" {
			input.ActorID = "mcp-agent"
		}
		if input.Reversibility == "" {
			input.Reversibility = events.Irreversible
		}
		evidence, err := json.Marshal(service.Redactor.Value(input.Evidence))
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		run, err := service.Store.GetRun(ctx, input.RunID)
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		agentAdapter, err := service.Adapters.Resolve(run.AgentType)
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		event, err := agentAdapter.Normalize(adapters.Action{RunID: input.RunID, ActionID: input.ActionID, ActorID: input.ActorID, Kind: input.Kind, Intent: service.Redactor.String(input.Intent), Evidence: events.Evidence{Data: evidence}, Reversibility: events.Reversibility{Status: input.Reversibility}})
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		risk, decision, err := service.Policy.Evaluate(ctx, event)
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		event.Risk, event.PolicyDecision = risk, decision
		sealed, err := service.Store.Append(ctx, event)
		if err != nil {
			return nil, recordActionOutput{}, err
		}
		return nil, recordActionOutput{EventID: sealed.EventID, Sequence: sealed.Sequence, IntegrityHash: sealed.IntegrityHash, Risk: risk.Level, PolicyStatus: decision.Status}, nil
	})
	mcp.AddTool(server, &mcp.Tool{Name: "get_run_summary", Description: "Get run counts and integrity status"}, func(ctx context.Context, _ *mcp.CallToolRequest, input runInput) (*mcp.CallToolResult, summaryOutput, error) {
		summary, err := service.Summary(ctx, input.RunID)
		return nil, summaryOutput{Summary: summary}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "request_rollback", Description: "Create a pending local rollback request"}, func(ctx context.Context, _ *mcp.CallToolRequest, input rollbackInput) (*mcp.CallToolResult, rollbackOutput, error) {
		if input.TargetEventID == "" && input.TargetSnapshotID == "" {
			return nil, rollbackOutput{}, errors.New("target_event_id or target_snapshot_id is required")
		}
		if input.TargetEventID != "" {
			item, err := service.Store.ByID(ctx, input.TargetEventID)
			if err != nil || item.RunID != input.RunID {
				return nil, rollbackOutput{}, errors.New("target event does not belong to run")
			}
		}
		if input.TargetSnapshotID != "" {
			item, err := service.Store.SnapshotByID(ctx, input.TargetSnapshotID)
			if err != nil || item.RunID != input.RunID {
				return nil, rollbackOutput{}, errors.New("target snapshot does not belong to run")
			}
		}
		plan, err := json.Marshal(map[string]any{"reason": service.Redactor.String(input.Reason)})
		if err != nil {
			return nil, rollbackOutput{}, err
		}
		record := events.RollbackRecord{RollbackID: events.NewID("rollback"), RunID: input.RunID, RequestedBy: "mcp-agent", TargetEventID: input.TargetEventID, TargetSnapshotID: input.TargetSnapshotID, Status: "pending", Plan: plan, RequestedAt: time.Now().UTC()}
		if err := service.Store.CreateRollback(ctx, record); err != nil {
			return nil, rollbackOutput{}, err
		}
		return nil, rollbackOutput{RollbackID: record.RollbackID, Status: record.Status}, nil
	})
	return server
}

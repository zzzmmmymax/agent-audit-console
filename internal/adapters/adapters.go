// Package adapters translates product identities into the stable event model.
package adapters

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

type Descriptor struct {
	Name, DisplayName, Version                                                                                   string
	Capabilities                                                                                                 []string
	SupportsSession, SupportsToolCalls, SupportsCommands, SupportsFileEvents, SupportsApproval, SupportsRollback bool
}
type Action struct {
	RunID, ActionID, ActorID, Intent string
	ParentActionID, CorrelationID    string
	Status                           ActionStatus
	Kind                             events.Kind
	Evidence                         events.Evidence
	Reversibility                    events.Reversibility
}
type ActionStatus = events.ActionStatus
type AgentAdapter interface {
	Descriptor() Descriptor
	NormalizeAgent(string) events.Actor
	NormalizeSession(map[string]string) map[string]string
	NormalizeToolCall(Action) (events.Event, error)
	NormalizeCommand(Action) (events.Event, error)
	NormalizeFileChange(Action) (events.Event, error)
	Normalize(Action) (events.Event, error)
}
type adapter struct{ descriptor Descriptor }

func (a adapter) Descriptor() Descriptor { return a.descriptor }
func (a adapter) NormalizeAgent(id string) events.Actor {
	if id == "" {
		id = a.descriptor.Name
	}
	return events.Actor{Type: "agent", ID: id, Name: a.descriptor.DisplayName, Metadata: map[string]string{"adapter": a.descriptor.Name, "adapter_version": a.descriptor.Version}}
}
func (a adapter) NormalizeSession(input map[string]string) map[string]string {
	result := map[string]string{"adapter": a.descriptor.Name}
	for key, value := range input {
		result[key] = value
	}
	return result
}
func (a adapter) NormalizeToolCall(action Action) (events.Event, error) {
	action.Kind = events.KindMCPCall
	return a.Normalize(action)
}
func (a adapter) NormalizeCommand(action Action) (events.Event, error) {
	action.Kind = events.KindCommand
	return a.Normalize(action)
}
func (a adapter) NormalizeFileChange(action Action) (events.Event, error) {
	action.Kind = events.KindFileChange
	return a.Normalize(action)
}
func (a adapter) Normalize(action Action) (events.Event, error) {
	if action.RunID == "" || action.Intent == "" {
		return events.Event{}, errors.New("run_id and intent are required")
	}
	if action.ActionID == "" {
		action.ActionID = events.NewID("action")
	}
	if action.ActorID == "" {
		action.ActorID = a.descriptor.Name
	}
	if action.Reversibility.Status == "" {
		action.Reversibility.Status = events.Irreversible
	}
	return events.Event{SchemaVersion: 2, EventID: events.NewID("event"), RunID: action.RunID, ActionID: action.ActionID, ParentActionID: action.ParentActionID, CorrelationID: action.CorrelationID, ActionStatus: action.Status, Timestamp: time.Now().UTC(), Actor: a.NormalizeAgent(action.ActorID), Kind: action.Kind, Intent: action.Intent, Evidence: action.Evidence, Risk: events.Risk{Level: events.RiskUnknown}, PolicyDecision: events.PolicyDecision{Status: events.PolicyNotEvaluated}, Reversibility: action.Reversibility}, nil
}

type Registry struct{ adapters map[string]AgentAdapter }

func Builtins() *Registry {
	r := &Registry{adapters: map[string]AgentAdapter{}}
	r.Register(adapter{Descriptor{Name: "codex", DisplayName: "Codex", Version: "2", Capabilities: []string{"cli", "mcp", "filesystem", "git"}, SupportsSession: true, SupportsToolCalls: true, SupportsCommands: true, SupportsFileEvents: true, SupportsApproval: true, SupportsRollback: true}})
	r.Register(adapter{Descriptor{Name: "claude-code", DisplayName: "Claude Code", Version: "2", Capabilities: []string{"cli", "mcp", "filesystem", "git"}, SupportsSession: true, SupportsToolCalls: true, SupportsCommands: true, SupportsFileEvents: true, SupportsApproval: true, SupportsRollback: true}})
	r.Register(adapter{Descriptor{Name: "cursor", DisplayName: "Cursor", Version: "2", Capabilities: []string{"mcp", "filesystem", "git"}, SupportsSession: true, SupportsToolCalls: true, SupportsFileEvents: true, SupportsApproval: true, SupportsRollback: true}})
	r.Register(adapter{Descriptor{Name: "custom", DisplayName: "Custom Agent", Version: "2", Capabilities: []string{"cli", "mcp"}, SupportsSession: true, SupportsToolCalls: true, SupportsCommands: true, SupportsApproval: true, SupportsRollback: true}})
	return r
}
func (r *Registry) Register(value AgentAdapter) {
	r.adapters[strings.ToLower(value.Descriptor().Name)] = value
}
func (r *Registry) Resolve(name string) (AgentAdapter, error) {
	if name == "" {
		name = "custom"
	}
	value, ok := r.adapters[strings.ToLower(name)]
	if !ok {
		return nil, errors.New("unknown agent adapter: " + name)
	}
	return value, nil
}
func (r *Registry) List() []Descriptor {
	result := make([]Descriptor, 0, len(r.adapters))
	for _, value := range r.adapters {
		result = append(result, value.Descriptor())
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

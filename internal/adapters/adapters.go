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
	Name, DisplayName string
	Capabilities      []string
}
type Action struct {
	RunID, ActionID, ActorID, Intent string
	Kind                             events.Kind
	Evidence                         events.Evidence
	Reversibility                    events.Reversibility
}
type AgentAdapter interface {
	Descriptor() Descriptor
	Normalize(Action) (events.Event, error)
}
type adapter struct{ descriptor Descriptor }

func (a adapter) Descriptor() Descriptor { return a.descriptor }
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
	return events.Event{EventID: events.NewID("event"), RunID: action.RunID, ActionID: action.ActionID, Timestamp: time.Now().UTC(), Actor: events.Actor{Type: "agent", ID: action.ActorID, Name: a.descriptor.DisplayName, Metadata: map[string]string{"adapter": a.descriptor.Name}}, Kind: action.Kind, Intent: action.Intent, Evidence: action.Evidence, Risk: events.Risk{Level: events.RiskUnknown}, PolicyDecision: events.PolicyDecision{Status: events.PolicyNotEvaluated}, Reversibility: action.Reversibility}, nil
}

type Registry struct{ adapters map[string]AgentAdapter }

func Builtins() *Registry {
	r := &Registry{adapters: map[string]AgentAdapter{}}
	r.Register(adapter{Descriptor{Name: "codex", DisplayName: "Codex", Capabilities: []string{"cli", "mcp", "filesystem", "git"}}})
	r.Register(adapter{Descriptor{Name: "claude-code", DisplayName: "Claude Code", Capabilities: []string{"cli", "mcp", "filesystem", "git"}}})
	r.Register(adapter{Descriptor{Name: "cursor", DisplayName: "Cursor", Capabilities: []string{"mcp", "filesystem", "git"}}})
	r.Register(adapter{Descriptor{Name: "custom", DisplayName: "Custom Agent", Capabilities: []string{"cli", "mcp"}}})
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

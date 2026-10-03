// Package events defines the product-neutral audit event model and its
// tamper-evident hash chain.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kind classifies an auditable operation independently of the originating
// agent product.
type Kind string

const (
	KindCommand    Kind = "command"
	KindFileChange Kind = "file_change"
	KindGit        Kind = "git"
	KindMCPCall    Kind = "mcp_call"
	KindApproval   Kind = "approval"
	KindRollback   Kind = "rollback"
)

// Actor identifies who or what caused an event. Product-specific attributes
// belong in Metadata so that the core model remains agent-neutral.
type Actor struct {
	Type     string            `json:"type"`
	ID       string            `json:"id"`
	Name     string            `json:"name,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Evidence contains observations supporting an event. Data is deliberately
// generic at the core boundary; kind-specific schemas can evolve separately.
type Evidence struct {
	Data       json.RawMessage `json:"data,omitempty"`
	References []string        `json:"references,omitempty"`
}

// Risk records the assessed severity and machine-readable reasons.
type Risk struct {
	Level   string   `json:"level"`
	Reasons []string `json:"reasons,omitempty"`
}

const (
	RiskUnknown = "unknown"
	RiskLow     = "low"
	RiskMedium  = "medium"
	RiskHigh    = "high"
)

// PolicyDecision records the result of policy evaluation. RuleID remains
// optional until the YAML policy engine is introduced in Stage 2.
type PolicyDecision struct {
	Status              string   `json:"status"`
	RuleID              string   `json:"rule_id,omitempty"`
	Reason              string   `json:"reason,omitempty"`
	Decision            string   `json:"decision,omitempty"`
	PolicySource        string   `json:"policy_source,omitempty"`
	PolicyFile          string   `json:"policy_file,omitempty"`
	RuleName            string   `json:"rule_name,omitempty"`
	RulePriority        int      `json:"rule_priority,omitempty"`
	MatchedRules        []string `json:"matched_rules,omitempty"`
	PolicyFingerprint   string   `json:"policy_fingerprint,omitempty"`
	PolicySchemaVersion int      `json:"policy_schema_version,omitempty"`
	RequiresApproval    bool     `json:"requires_approval,omitempty"`
}

const (
	PolicyNotEvaluated = "not_evaluated"
	PolicyAllowed      = "allowed"
	PolicyPending      = "pending"
	PolicyApproved     = "approved"
	PolicyRejected     = "rejected"
)

// Reversibility describes whether the event's effect can be restored.
type Reversibility struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

const (
	Reversible       = "reversible"
	PartlyReversible = "partially_reversible"
	Irreversible     = "irreversible"
	NotApplicable    = "not_applicable"
)

// Event is the immutable audit record persisted by the system. PreviousHash
// and IntegrityHash link events within a run. Hashes are lowercase SHA-256 hex.
type Event struct {
	SchemaVersion  int            `json:"schema_version,omitempty"`
	EventID        string         `json:"event_id"`
	RunID          string         `json:"run_id"`
	ActionID       string         `json:"action_id"`
	ParentActionID string         `json:"parent_action_id,omitempty"`
	CorrelationID  string         `json:"correlation_id,omitempty"`
	ActionStatus   ActionStatus   `json:"action_status,omitempty"`
	Timestamp      time.Time      `json:"timestamp"`
	Sequence       uint64         `json:"sequence"`
	Actor          Actor          `json:"actor"`
	Kind           Kind           `json:"kind"`
	Intent         string         `json:"intent"`
	Evidence       Evidence       `json:"evidence"`
	Risk           Risk           `json:"risk"`
	PolicyDecision PolicyDecision `json:"policy_decision"`
	Reversibility  Reversibility  `json:"reversibility"`
	PreviousHash   string         `json:"previous_hash"`
	IntegrityHash  string         `json:"integrity_hash"`
}

var validKinds = map[Kind]struct{}{
	KindCommand: {}, KindFileChange: {}, KindGit: {}, KindMCPCall: {},
	KindApproval: {}, KindRollback: {},
}

var validRiskLevels = stringSet(RiskUnknown, RiskLow, RiskMedium, RiskHigh)
var validPolicyStatuses = stringSet(PolicyNotEvaluated, PolicyAllowed, PolicyPending, PolicyApproved, PolicyRejected)
var validReversibilityStatuses = stringSet(Reversible, PartlyReversible, Irreversible, NotApplicable)

func stringSet(values ...string) map[string]struct{} {
	result := make(map[string]struct{}, len(values))
	for _, value := range values {
		result[value] = struct{}{}
	}
	return result
}

// Validate checks invariants required before an event can be sealed or stored.
func (e Event) Validate() error {
	var problems []string
	if strings.TrimSpace(e.EventID) == "" {
		problems = append(problems, "event_id is required")
	}
	if strings.TrimSpace(e.RunID) == "" {
		problems = append(problems, "run_id is required")
	}
	if strings.TrimSpace(e.ActionID) == "" {
		problems = append(problems, "action_id is required")
	}
	if e.SchemaVersion != 0 && e.SchemaVersion != 1 && e.SchemaVersion != 2 {
		problems = append(problems, fmt.Sprintf("unsupported schema_version %d", e.SchemaVersion))
	}
	if e.ActionStatus != "" && !ValidActionStatus(e.ActionStatus) {
		problems = append(problems, fmt.Sprintf("unsupported action_status %q", e.ActionStatus))
	}
	if e.Timestamp.IsZero() {
		problems = append(problems, "timestamp is required")
	}
	if e.Sequence == 0 {
		problems = append(problems, "sequence must be at least 1")
	}
	if strings.TrimSpace(e.Actor.Type) == "" || strings.TrimSpace(e.Actor.ID) == "" {
		problems = append(problems, "actor.type and actor.id are required")
	}
	if _, ok := validKinds[e.Kind]; !ok {
		problems = append(problems, fmt.Sprintf("unsupported kind %q", e.Kind))
	}
	if len(e.Evidence.Data) > 0 && !json.Valid(e.Evidence.Data) {
		problems = append(problems, "evidence.data must be valid JSON")
	}
	if _, ok := validRiskLevels[e.Risk.Level]; !ok {
		problems = append(problems, fmt.Sprintf("unsupported risk level %q", e.Risk.Level))
	}
	if _, ok := validPolicyStatuses[e.PolicyDecision.Status]; !ok {
		problems = append(problems, fmt.Sprintf("unsupported policy status %q", e.PolicyDecision.Status))
	}
	if _, ok := validReversibilityStatuses[e.Reversibility.Status]; !ok {
		problems = append(problems, fmt.Sprintf("unsupported reversibility status %q", e.Reversibility.Status))
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func ValidActionStatus(value ActionStatus) bool {
	switch value {
	case ActionPlanned, ActionStarted, ActionCompleted, ActionFailed, ActionBlocked, ActionCancelled:
		return true
	default:
		return false
	}
}

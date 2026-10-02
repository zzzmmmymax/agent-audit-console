// Package policy defines the risk evaluation boundary. YAML rules are Stage 2.
package policy

import (
	"context"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

// Evaluator assigns risk and a policy decision before execution.
type Evaluator interface {
	Evaluate(context.Context, events.Event) (events.Risk, events.PolicyDecision, error)
}

// Package rollback defines recovery planning and execution boundaries.
package rollback

import "context"

// Plan describes a reviewable recovery action without executing it.
type Plan struct {
	ID, RunID, TargetEventID, TargetSnapshotID, Summary string
}

// Planner creates plans separately from their execution.
type Planner interface {
	Plan(context.Context, string, string) (Plan, error)
}

// Executor applies a previously reviewed rollback plan.
type Executor interface {
	Execute(context.Context, Plan) error
}

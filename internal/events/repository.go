package events

import "context"

// Query limits and filters event retrieval without binding the domain layer to
// SQLite or an HTTP representation.
type Query struct {
	RunID string
	Kinds []Kind
	After uint64
	Limit int
}

// Repository is the persistence boundary for immutable event records.
// Implementations must append an event and advance its run head atomically.
type Repository interface {
	Append(context.Context, Event) (Event, error)
	ByID(context.Context, string) (Event, error)
	Query(context.Context, Query) ([]Event, error)
	VerifyRun(context.Context, string) error
}

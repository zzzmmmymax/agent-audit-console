// Package capture defines the Stage 1 boundary for collecting shell, file, and
// Git evidence. Stage 0 provides contracts only.
package capture

import (
	"context"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

// Sink accepts captured, unsealed events. The orchestration layer assigns
// sequence numbers and seals/persists them transactionally.
type Sink interface {
	Record(context.Context, events.Event) error
}

// Collector observes one controlled agent operation source.
type Collector interface {
	Start(context.Context, Sink) error
	Close() error
}

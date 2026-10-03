package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func BenchmarkStartRun(b *testing.B) {
	service, err := audit.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	workspace := b.TempDir()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: fmt.Sprint(i), WorkspacePath: workspace}}); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRecordAction(b *testing.B) {
	service, err := audit.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	run, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "bench", WorkspacePath: b.TempDir()}})
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := writeAction(ctx, service, actionInput{runRef: runRef{RunID: run.RunID}, ActionID: fmt.Sprintf("a-%d", i), Kind: events.KindMCPCall, Intent: "benchmark"}, events.ActionStarted)
		if err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkRunSummary(b *testing.B) {
	service, run := benchmarkRun(b, 100)
	ctx := context.Background()
	defer service.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Summary(ctx, run); err != nil {
			b.Fatal(err)
		}
	}
}
func BenchmarkListEvents100(b *testing.B)  { benchmarkListEvents(b, 100) }
func BenchmarkListEvents1000(b *testing.B) { benchmarkListEvents(b, 1000) }
func benchmarkListEvents(b *testing.B, count int) {
	service, run := benchmarkRun(b, count)
	ctx := context.Background()
	defer service.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Store.Query(ctx, events.Query{RunID: run, Limit: count}); err != nil {
			b.Fatal(err)
		}
	}
}
func benchmarkRun(b *testing.B, count int) (*audit.Service, string) {
	b.Helper()
	service, err := audit.Open(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	run, err := startRun(ctx, service, startRunInput{runRef: runRef{AgentType: "custom", SessionID: "bench", WorkspacePath: b.TempDir()}})
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < count; i++ {
		if _, err := writeAction(ctx, service, actionInput{runRef: runRef{RunID: run.RunID}, ActionID: fmt.Sprintf("a-%d", i), Kind: events.KindMCPCall, Intent: "benchmark"}, events.ActionStarted); err != nil {
			b.Fatal(err)
		}
	}
	return service, run.RunID
}

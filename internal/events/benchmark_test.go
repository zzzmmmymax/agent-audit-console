package events

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func BenchmarkWrite10000Events(b *testing.B) {
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		store, runID := benchmarkStore(b, 0)
		b.StartTimer()
		appendBenchmarkEvents(b, store, runID, 10000)
		b.StopTimer()
		store.Close()
	}
}

func BenchmarkQuery10000Events(b *testing.B) {
	store, runID := benchmarkStore(b, 10000)
	defer store.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		items, err := store.Query(context.Background(), Query{RunID: runID, Limit: 10000})
		if err != nil || len(items) != 10000 {
			b.Fatalf("events=%d err=%v", len(items), err)
		}
	}
}

func BenchmarkVerify10000Events(b *testing.B) {
	store, runID := benchmarkStore(b, 10000)
	defer store.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := store.VerifyRun(context.Background(), runID); err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkStore(b *testing.B, count int) (*SQLiteStore, string) {
	b.Helper()
	store, err := OpenSQLite(filepath.Join(b.TempDir(), NewID("benchmark")+".db"))
	if err != nil {
		b.Fatal(err)
	}
	runID := NewID("run")
	run := Run{RunID: runID, AgentType: "custom", AgentID: "benchmark", Status: "running", WorkspacePath: b.TempDir(), StartedAt: time.Now().UTC(), Metadata: map[string]string{}}
	if err := store.CreateRun(context.Background(), run); err != nil {
		b.Fatal(err)
	}
	appendBenchmarkEvents(b, store, runID, count)
	return store, runID
}

func appendBenchmarkEvents(b *testing.B, store *SQLiteStore, runID string, count int) {
	b.Helper()
	for i := 0; i < count; i++ {
		event := testEvent(1)
		event.EventID, event.RunID, event.Sequence, event.Timestamp = NewID("event"), runID, 0, time.Now().UTC()
		if _, err := store.Append(context.Background(), event); err != nil {
			b.Fatal(err)
		}
	}
}

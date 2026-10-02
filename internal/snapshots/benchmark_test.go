package snapshots

import (
	"bytes"
	"context"
	"testing"
)

func BenchmarkSnapshotStore1MiB(b *testing.B) {
	store, err := NewDiskStore(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	payload := bytes.Repeat([]byte("audit-data-"), 95325)
	b.SetBytes(int64(len(payload)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := store.Put(context.Background(), Metadata{}, bytes.NewReader(payload)); err != nil {
			b.Fatal(err)
		}
	}
}

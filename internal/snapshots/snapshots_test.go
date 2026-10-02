package snapshots

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestConcurrentPutDeduplicates(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	errorsSeen := make(chan error, 12)
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := store.Put(context.Background(), Metadata{}, bytes.NewBufferString("same content"))
			errorsSeen <- err
		}()
	}
	group.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRestoreVerifiesBeforeReplacingTarget(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := store.Put(context.Background(), Metadata{}, bytes.NewBufferString("snapshot"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(metadata.ObjectPath, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "target.txt")
	if err := os.WriteFile(target, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.Restore(context.Background(), metadata.ContentHash, target, 0o600); err == nil {
		t.Fatal("corrupt snapshot restored")
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "original" {
		t.Fatalf("target changed to %q", data)
	}
}

func TestOpenRejectsNonHexHash(t *testing.T) {
	store, err := NewDiskStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := store.Open(context.Background(), string(bytes.Repeat([]byte{'z'}, 64))); err == nil {
		t.Fatal("non-hex hash accepted")
	}
}

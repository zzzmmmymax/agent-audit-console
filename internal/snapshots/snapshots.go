// Package snapshots defines content-addressed file snapshot contracts.
package snapshots

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Metadata identifies an immutable SHA-256 addressed snapshot.
type Metadata struct {
	ID, RunID, ActionID, FilePath, ContentHash, ObjectPath string
	ByteSize                                               int64
	FileMode                                               uint32
}

// Store persists and verifies immutable snapshot content.
type Store interface {
	Put(context.Context, Metadata, io.Reader) (Metadata, error)
	Open(context.Context, string) (io.ReadCloser, Metadata, error)
	Verify(context.Context, string) error
}

type DiskStore struct {
	root string
	mu   sync.Mutex
}

func NewDiskStore(root string) (*DiskStore, error) {
	if err := os.MkdirAll(filepath.Join(root, "objects"), 0o700); err != nil {
		return nil, err
	}
	return &DiskStore{root: root}, nil
}

func (s *DiskStore) Put(ctx context.Context, metadata Metadata, source io.Reader) (Metadata, error) {
	temporary, err := os.CreateTemp(s.root, "snapshot-*")
	if err != nil {
		return metadata, err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(temporary, hasher), &contextReader{ctx: ctx, reader: source})
	if syncErr := temporary.Sync(); err == nil {
		err = syncErr
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return metadata, err
	}
	hash := hex.EncodeToString(hasher.Sum(nil))
	destination := filepath.Join(s.root, "objects", hash[:2], hash)
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return metadata, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(destination); err != nil {
		if !os.IsNotExist(err) {
			return metadata, err
		}
		if err := os.Rename(temporaryName, destination); err != nil {
			if verifyErr := s.verifyWithRetry(ctx, hash); verifyErr != nil {
				return metadata, err
			}
		}
	}
	metadata.ContentHash, metadata.ObjectPath, metadata.ByteSize = hash, destination, written
	return metadata, nil
}

func (s *DiskStore) verifyWithRetry(ctx context.Context, hash string) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		if err = s.Verify(ctx, hash); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return err
}

func (s *DiskStore) Open(ctx context.Context, hash string) (io.ReadCloser, Metadata, error) {
	if !validHash(hash) {
		return nil, Metadata{}, fmt.Errorf("invalid snapshot hash")
	}
	path := filepath.Join(s.root, "objects", hash[:2], hash)
	file, err := os.Open(path)
	if err != nil {
		return nil, Metadata{}, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, Metadata{}, err
	}
	return file, Metadata{ContentHash: hash, ObjectPath: path, ByteSize: info.Size()}, nil
}

func validHash(hash string) bool {
	if len(hash) != 64 || hash != strings.ToLower(hash) {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func (s *DiskStore) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	probe, err := os.CreateTemp(s.root, ".ready-*")
	if err != nil {
		return err
	}
	name := probe.Name()
	if err := probe.Close(); err != nil {
		return err
	}
	return os.Remove(name)
}

func (s *DiskStore) Verify(ctx context.Context, hash string) error {
	reader, _, err := s.Open(ctx, hash)
	if err != nil {
		return err
	}
	defer reader.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, &contextReader{ctx: ctx, reader: reader}); err != nil {
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != hash {
		return fmt.Errorf("snapshot integrity mismatch")
	}
	return nil
}

func (s *DiskStore) Restore(ctx context.Context, hash, target string, mode os.FileMode) error {
	if err := s.Verify(ctx, hash); err != nil {
		return err
	}
	reader, _, err := s.Open(ctx, hash)
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(target), ".audit-restore-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if _, err := io.Copy(temporary, &contextReader{ctx: ctx, reader: reader}); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Chmod(name, mode); err != nil {
		return err
	}
	if _, err := os.Stat(target); err == nil {
		placeholder, createErr := os.CreateTemp(filepath.Dir(target), ".audit-backup-*")
		if createErr != nil {
			return createErr
		}
		backup := placeholder.Name()
		if closeErr := placeholder.Close(); closeErr != nil {
			return closeErr
		}
		if removeErr := os.Remove(backup); removeErr != nil {
			return removeErr
		}
		if err := os.Rename(target, backup); err != nil {
			return err
		}
		if err := os.Rename(name, target); err != nil {
			_ = os.Rename(backup, target)
			return err
		}
		if err := os.Remove(backup); err != nil {
			return err
		}
	} else if os.IsNotExist(err) {
		if err := os.Rename(name, target); err != nil {
			return err
		}
	} else {
		return err
	}
	return nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}

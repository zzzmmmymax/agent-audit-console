package capture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agent-audit-console/agent-audit-console/internal/snapshots"
	"github.com/pmezard/go-difflib/difflib"
)

type FileState struct {
	Path, RelativePath, Hash, ObjectPath string
	Size                                 int64
	Mode                                 os.FileMode
}
type WorkspaceState map[string]FileState
type Change struct {
	Type          string
	Before, After *FileState
	Diff          string
}

func ScanWorkspace(ctx context.Context, root string, store *snapshots.DiskStore) (WorkspaceState, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	result := WorkspaceState{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == ".agent-audit" || entry.Name() == "node_modules") && path != root {
			return filepath.SkipDir
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		meta, err := store.Put(ctx, snapshots.Metadata{FilePath: path}, file)
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		result[path] = FileState{Path: path, RelativePath: filepath.ToSlash(relative), Hash: meta.ContentHash, ObjectPath: meta.ObjectPath, Size: info.Size(), Mode: info.Mode()}
		return nil
	})
	return result, err
}

func CompareWorkspace(before, after WorkspaceState) ([]Change, error) {
	keys := map[string]struct{}{}
	for key := range before {
		keys[key] = struct{}{}
	}
	for key := range after {
		keys[key] = struct{}{}
	}
	ordered := make([]string, 0, len(keys))
	for key := range keys {
		ordered = append(ordered, key)
	}
	sort.Strings(ordered)
	var changes []Change
	for _, path := range ordered {
		old, oldOK := before[path]
		current, newOK := after[path]
		if oldOK && newOK && old.Hash == current.Hash {
			continue
		}
		change := Change{}
		switch {
		case !oldOK:
			change.Type = "added"
			change.After = &current
		case !newOK:
			change.Type = "deleted"
			change.Before = &old
		default:
			change.Type = "modified"
			change.Before = &old
			change.After = &current
		}
		diff, err := makeDiff(change)
		if err != nil {
			return nil, err
		}
		change.Diff = diff
		changes = append(changes, change)
	}
	return changes, nil
}

func makeDiff(change Change) (string, error) {
	var oldName, newName string
	var oldData, newData []byte
	if change.Before != nil {
		oldName = "a/" + change.Before.RelativePath
		var err error
		oldData, err = os.ReadFile(change.Before.ObjectPath)
		if err != nil {
			return "", err
		}
	} else {
		oldName = "/dev/null"
	}
	if change.After != nil {
		newName = "b/" + change.After.RelativePath
		var err error
		newData, err = os.ReadFile(change.After.ObjectPath)
		if err != nil {
			return "", err
		}
	} else {
		newName = "/dev/null"
	}
	if bytes.IndexByte(oldData, 0) >= 0 || bytes.IndexByte(newData, 0) >= 0 {
		return fmt.Sprintf("Binary files %s and %s differ", oldName, newName), nil
	}
	diff, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{A: difflib.SplitLines(string(oldData)), B: difflib.SplitLines(string(newData)), FromFile: oldName, ToFile: newName, Context: 3})
	return diff, err
}

func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func IsWithin(root, target string) bool {
	relative, err := filepath.Rel(root, target)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

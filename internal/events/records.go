package events

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

type Run struct {
	RunID         string            `json:"run_id"`
	AgentType     string            `json:"agent_type"`
	AgentID       string            `json:"agent_id"`
	Status        string            `json:"status"`
	WorkspacePath string            `json:"workspace_path"`
	StartedAt     time.Time         `json:"started_at"`
	EndedAt       *time.Time        `json:"ended_at,omitempty"`
	HeadHash      string            `json:"head_hash"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type SnapshotRecord struct {
	SnapshotID, RunID, ActionID, FilePath string
	Exists                                bool
	ExpectedExists                        *bool
	ContentHash, ObjectPath               string
	ExpectedHash                          string
	ByteSize                              int64
	FileMode                              uint32
	CapturedAt                            time.Time
	VerifiedAt                            *time.Time
}

type RollbackRecord struct {
	RollbackID, RunID, RequestedBy, TargetEventID, TargetSnapshotID, Status string
	Plan, Result                                                            json.RawMessage
	RequestedAt                                                             time.Time
	CompletedAt                                                             *time.Time
}

func NewID(prefix string) string {
	var value [12]byte
	if _, err := rand.Read(value[:]); err == nil {
		return prefix + "_" + time.Now().UTC().Format("20060102T150405.000000000") + "_" + hex.EncodeToString(value[:])
	}
	sequence := fallbackIDCounter.Add(1)
	return fmt.Sprintf("%s_%s_fallback_%d_%d", prefix, time.Now().UTC().Format("20060102T150405.000000000"), os.Getpid(), sequence)
}

var fallbackIDCounter atomic.Uint64

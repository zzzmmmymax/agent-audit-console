package events

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrIntegrityMismatch indicates that an event's stored hash does not match
// its content or predecessor.
var ErrIntegrityMismatch = errors.New("event integrity hash mismatch")

// hashPayload fixes the hashed field order and excludes IntegrityHash itself.
// encoding/json sorts map keys, giving deterministic output for Actor.Metadata.
type hashPayload struct {
	EventID        string         `json:"event_id"`
	RunID          string         `json:"run_id"`
	ActionID       string         `json:"action_id"`
	Timestamp      string         `json:"timestamp"`
	Sequence       uint64         `json:"sequence"`
	Actor          Actor          `json:"actor"`
	Kind           Kind           `json:"kind"`
	Intent         string         `json:"intent"`
	EvidenceDigest string         `json:"evidence_digest"`
	References     []string       `json:"references,omitempty"`
	Risk           Risk           `json:"risk"`
	PolicyDecision PolicyDecision `json:"policy_decision"`
	Reversibility  Reversibility  `json:"reversibility"`
	PreviousHash   string         `json:"previous_hash"`
}

// ComputeIntegrityHash calculates an event hash from all material event fields,
// including PreviousHash. The event is not mutated.
func ComputeIntegrityHash(event Event) (string, error) {
	if err := event.Validate(); err != nil {
		return "", err
	}
	evidenceDigest, err := canonicalEvidenceDigest(event.Evidence.Data)
	if err != nil {
		return "", err
	}
	payload := hashPayload{
		EventID: event.EventID, RunID: event.RunID, ActionID: event.ActionID,
		Timestamp: event.Timestamp.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		Sequence:  event.Sequence, Actor: event.Actor, Kind: event.Kind,
		Intent: event.Intent, EvidenceDigest: evidenceDigest, References: event.Evidence.References, Risk: event.Risk,
		PolicyDecision: event.PolicyDecision, Reversibility: event.Reversibility,
		PreviousHash: event.PreviousHash,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal event hash payload: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func canonicalEvidenceDigest(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", fmt.Errorf("decode evidence for hashing: %w", err)
	}
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("canonicalize evidence for hashing: %w", err)
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

// computeLegacyIntegrityHash keeps pre-v0.1 development databases verifiable.
// New events always use the canonical evidence digest above.
func computeLegacyIntegrityHash(event Event) (string, error) {
	if err := event.Validate(); err != nil {
		return "", err
	}
	type legacyPayload struct {
		EventID        string         `json:"event_id"`
		RunID          string         `json:"run_id"`
		ActionID       string         `json:"action_id"`
		Timestamp      string         `json:"timestamp"`
		Sequence       uint64         `json:"sequence"`
		Actor          Actor          `json:"actor"`
		Kind           Kind           `json:"kind"`
		Intent         string         `json:"intent"`
		Evidence       Evidence       `json:"evidence"`
		Risk           Risk           `json:"risk"`
		PolicyDecision PolicyDecision `json:"policy_decision"`
		Reversibility  Reversibility  `json:"reversibility"`
		PreviousHash   string         `json:"previous_hash"`
	}
	payload := legacyPayload{event.EventID, event.RunID, event.ActionID, event.Timestamp.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), event.Sequence, event.Actor, event.Kind, event.Intent, event.Evidence, event.Risk, event.PolicyDecision, event.Reversibility, event.PreviousHash}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func hashMatches(event Event) (bool, error) {
	expected, err := ComputeIntegrityHash(event)
	if err != nil {
		return false, err
	}
	if event.IntegrityHash == expected {
		return true, nil
	}
	legacy, err := computeLegacyIntegrityHash(event)
	return err == nil && event.IntegrityHash == legacy, err
}

// Seal sets the predecessor and current integrity hashes on an event copy.
func Seal(event Event, previousHash string) (Event, error) {
	if previousHash != "" {
		if !validDigest(previousHash) {
			return Event{}, errors.New("previous hash must be a SHA-256 hex digest")
		}
	}
	event.PreviousHash = strings.ToLower(previousHash)
	event.IntegrityHash = ""
	currentHash, err := ComputeIntegrityHash(event)
	if err != nil {
		return Event{}, err
	}
	event.IntegrityHash = currentHash
	return event, nil
}

// VerifyChain verifies a complete run: genesis identity, run identity,
// contiguous sequence numbers, predecessor links, and every content hash.
func VerifyChain(chain []Event) error {
	if len(chain) == 0 {
		return nil
	}
	if chain[0].Sequence != 1 || chain[0].PreviousHash != "" {
		return errors.New("event 0: invalid genesis event")
	}
	runID := chain[0].RunID
	for i, event := range chain {
		if event.RunID != runID {
			return fmt.Errorf("event %d: run_id changed", i)
		}
		if i > 0 {
			previous := chain[i-1]
			if event.Sequence != previous.Sequence+1 {
				return fmt.Errorf("event %d: non-contiguous sequence", i)
			}
			if event.PreviousHash != previous.IntegrityHash {
				return fmt.Errorf("event %d: previous hash mismatch", i)
			}
		}
		matches, err := hashMatches(event)
		if err != nil {
			return fmt.Errorf("event %d: %w", i, err)
		}
		if !validDigest(event.IntegrityHash) || !matches {
			return fmt.Errorf("event %d: %w", i, ErrIntegrityMismatch)
		}
	}
	return nil
}

func validDigest(value string) bool {
	if value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

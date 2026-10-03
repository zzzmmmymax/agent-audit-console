package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

func testEvent(sequence uint64) Event {
	return Event{
		EventID: fmt.Sprintf("event-%d", sequence), RunID: "run-1",
		ActionID: "action-1", Timestamp: time.Date(2026, 10, 1, 12, 0, int(sequence), 0, time.FixedZone("CST", 8*60*60)),
		Sequence: sequence, Actor: Actor{Type: "agent", ID: "codex"},
		Kind: KindCommand, Intent: "run tests",
		Evidence: Evidence{Data: json.RawMessage(`{"command":"go test ./..."}`)},
		Risk:     Risk{Level: RiskLow}, PolicyDecision: PolicyDecision{Status: PolicyAllowed},
		Reversibility: Reversibility{Status: NotApplicable},
	}
}

func TestSealAndVerifyChain(t *testing.T) {
	first, err := Seal(testEvent(1), "")
	if err != nil {
		t.Fatal(err)
	}
	second, err := Seal(testEvent(2), first.IntegrityHash)
	if err != nil {
		t.Fatal(err)
	}
	if second.PreviousHash != first.IntegrityHash {
		t.Fatal("second event does not reference first")
	}
	if err := VerifyChain([]Event{first, second}); err != nil {
		t.Fatalf("valid chain rejected: %v", err)
	}
}

func TestVerifyChainDetectsMutation(t *testing.T) {
	first, err := Seal(testEvent(1), "")
	if err != nil {
		t.Fatal(err)
	}
	first.Intent = "tampered intent"
	err = VerifyChain([]Event{first})
	if !errors.Is(err, ErrIntegrityMismatch) {
		t.Fatalf("expected integrity mismatch, got %v", err)
	}
}

func TestVerifyChainDetectsBrokenLink(t *testing.T) {
	first, _ := Seal(testEvent(1), "")
	second, _ := Seal(testEvent(2), "")
	if err := VerifyChain([]Event{first, second}); err == nil {
		t.Fatal("expected broken link error")
	}
}

func TestHashIsStableAcrossTimeZonesAndMapOrder(t *testing.T) {
	left := testEvent(1)
	left.Actor.Metadata = map[string]string{"z": "last", "a": "first"}
	right := left
	right.Timestamp = left.Timestamp.UTC()
	right.Actor.Metadata = map[string]string{"a": "first", "z": "last"}
	leftHash, err := ComputeIntegrityHash(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := ComputeIntegrityHash(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("hashes differ: %s != %s", leftHash, rightHash)
	}
}

func TestHashCanonicalizesEvidenceObjectOrder(t *testing.T) {
	left := testEvent(1)
	right := testEvent(1)
	left.Evidence.Data = json.RawMessage(`{"z":1,"nested":{"b":2,"a":1}}`)
	right.Evidence.Data = json.RawMessage(`{ "nested": {"a":1,"b":2}, "z": 1 }`)
	leftHash, err := ComputeIntegrityHash(left)
	if err != nil {
		t.Fatal(err)
	}
	rightHash, err := ComputeIntegrityHash(right)
	if err != nil {
		t.Fatal(err)
	}
	if leftHash != rightHash {
		t.Fatalf("canonical hashes differ: %s != %s", leftHash, rightHash)
	}
}

func TestSealRejectsInvalidPreviousHash(t *testing.T) {
	if _, err := Seal(testEvent(1), "not-a-hash"); err == nil {
		t.Fatal("expected invalid previous hash error")
	}
}

func TestVerifyChainRejectsNonGenesisStart(t *testing.T) {
	event := testEvent(2)
	event, err := Seal(event, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyChain([]Event{event}); err == nil {
		t.Fatal("expected invalid genesis error")
	}
}

func TestLegacyDevelopmentHashRemainsVerifiable(t *testing.T) {
	event := testEvent(1)
	legacy, err := computeLegacyIntegrityHash(event)
	if err != nil {
		t.Fatal(err)
	}
	event.IntegrityHash = legacy
	if err := VerifyChain([]Event{event}); err != nil {
		t.Fatalf("legacy chain rejected: %v", err)
	}
}

func TestV2HashCoversCorrelationFields(t *testing.T) {
	event := testEvent(1)
	event.SchemaVersion = 2
	event.CorrelationID = "original"
	sealed, err := Seal(event, "")
	if err != nil {
		t.Fatal(err)
	}
	sealed.CorrelationID = "tampered"
	if err := VerifyChain([]Event{sealed}); err == nil {
		t.Fatal("v2 correlation tampering was not detected")
	}
}

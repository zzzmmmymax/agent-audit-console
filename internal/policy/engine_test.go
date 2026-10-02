package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func TestDefaultRiskRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := EnsureDefault(path); err != nil {
		t.Fatal(err)
	}
	engine, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		command      []string
		risk, status string
	}{{[]string{"git", "push", "origin", "main"}, events.RiskHigh, events.PolicyPending}, {[]string{"curl", "https://example.com"}, events.RiskMedium, events.PolicyAllowed}, {[]string{"go", "test", "./..."}, events.RiskLow, events.PolicyAllowed}}
	for _, item := range cases {
		data, _ := json.Marshal(map[string]any{"command": item.command})
		event := events.Event{Timestamp: time.Now(), Kind: events.KindCommand, Evidence: events.Evidence{Data: data}}
		risk, decision, err := engine.Evaluate(context.Background(), event)
		if err != nil {
			t.Fatal(err)
		}
		if risk.Level != item.risk || decision.Status != item.status {
			t.Errorf("%v => %s/%s", item.command, risk.Level, decision.Status)
		}
	}
}

func TestTeamPolicyTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.yaml")
	team := filepath.Join(dir, "team.yaml")
	base := `version: 1
default: {risk: low, decision: allowed}
rules:
  - id: local-allow
    command_regex: 'deploy'
    risk: low
    decision: allowed
`
	shared := `version: 1
default: {risk: low, decision: allowed}
rules:
  - id: team-block
    description: team deployment gate
    command_regex: 'deploy'
    risk: high
    decision: pending
`
	if err := os.WriteFile(local, []byte(base), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(team, []byte(shared), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := LoadLayered(local, team)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"command": []string{"deploy"}})
	risk, decision, err := engine.Evaluate(context.Background(), events.Event{Kind: events.KindCommand, Evidence: events.Evidence{Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if risk.Level != events.RiskHigh || decision.RuleID != "team-block" {
		t.Fatalf("risk=%s rule=%s", risk.Level, decision.RuleID)
	}
	if len(engine.Sources()) != 2 {
		t.Fatalf("sources=%v", engine.Sources())
	}
}

func TestTeamDefaultCannotBeWeakenedLocally(t *testing.T) {
	dir := t.TempDir()
	local := filepath.Join(dir, "local.yaml")
	team := filepath.Join(dir, "team.yaml")
	if err := os.WriteFile(local, []byte("version: 1\ndefault: {risk: low, decision: allowed}\nrules: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(team, []byte("version: 1\ndefault: {risk: medium, decision: pending}\nrules: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := LoadLayered(local, team)
	if err != nil {
		t.Fatal(err)
	}
	risk, decision, err := engine.Evaluate(context.Background(), events.Event{Kind: events.KindCommand})
	if err != nil {
		t.Fatal(err)
	}
	if risk.Level != events.RiskMedium || decision.Status != events.PolicyPending {
		t.Fatalf("default=%s/%s", risk.Level, decision.Status)
	}
}

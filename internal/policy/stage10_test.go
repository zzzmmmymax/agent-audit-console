package policy

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
)

func policyFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecisionStrictnessAndRiskSeparation(t *testing.T) {
	for index, decision := range []string{DecisionAllow, DecisionWarn, DecisionApproval, DecisionDeny} {
		if DecisionRank(decision) != index+1 {
			t.Fatalf("rank(%s)=%d", decision, DecisionRank(decision))
		}
	}
	path := policyFile(t, "version: 1\ndefault: {risk: low, decision: allow}\nrules:\n- {id: a, command_regex: x, risk: high, decision: warn}\n- {id: b, command_regex: x, risk: low, decision: require_approval}\n")
	engine, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"command": []string{"x"}})
	result, err := engine.Simulate(context.Background(), events.Event{Kind: events.KindCommand, Evidence: events.Evidence{Data: data}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionApproval || result.RiskLevel != events.RiskHigh || len(result.MatchedRules) != 2 || len(result.Trace) < 4 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestValidationAndCanonicalFingerprint(t *testing.T) {
	bad := ValidateDocument([]byte("version: 1\ndefault: {risk: low, decision: allow}\nrules:\n- {id: x, risk: low, decision: nope}\n- {id: x, risk: low, decision: allow}\n"))
	if bad.Valid || len(bad.Errors) < 2 {
		t.Fatalf("report=%+v", bad)
	}
	one := []byte("version: 1\ndefault:\n  risk: low\n  decision: allow\nrules:\n- id: a\n  risk: high\n  decision: deny\n")
	two := []byte("rules:\r\n  - decision: deny\r\n    risk: high\r\n    id: a\r\ndefault: {decision: allow, risk: low}\r\nversion: 1\r\n")
	a, b := ValidateDocument(one), ValidateDocument(two)
	if !a.Valid || !b.Valid || a.Fingerprint != b.Fingerprint {
		t.Fatalf("fingerprints %s %s", a.Fingerprint, b.Fingerprint)
	}
}

func TestSimulationDeterminismAndNoSideEffects(t *testing.T) {
	path := policyFile(t, "version: 1\ndefault: {risk: low, decision: allow}\nrules:\n- {id: b, command_regex: x, risk: medium, decision: warn}\n- {id: a, command_regex: x, risk: high, decision: deny}\n")
	engine, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	data, _ := json.Marshal(map[string]any{"command": []string{"x"}})
	first, _ := engine.Simulate(context.Background(), events.Event{Kind: events.KindCommand, Evidence: events.Evidence{Data: data}})
	second, _ := engine.Simulate(context.Background(), events.Event{Kind: events.KindCommand, Evidence: events.Evidence{Data: data}})
	oneJSON, _ := json.Marshal(first)
	twoJSON, _ := json.Marshal(second)
	after, _ := os.ReadFile(path)
	if string(oneJSON) != string(twoJSON) || string(before) != string(after) {
		t.Fatal("simulation was not deterministic/read-only")
	}
}

func TestLastKnownGood(t *testing.T) {
	path := policyFile(t, "version: 1\ndefault: {risk: low, decision: allow}\nrules: []\n")
	engine, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint := engine.Fingerprint()
	if err := os.WriteFile(path, []byte("not: valid"), 0o600); err != nil {
		t.Fatal(err)
	}
	if engine.Reload() == nil || engine.Fingerprint() != fingerprint || !strings.HasPrefix(engine.Health(), "WARN") {
		t.Fatalf("health=%s", engine.Health())
	}
}

func TestPolicyTestRunner(t *testing.T) {
	path := policyFile(t, "version: 1\ndefault: {risk: low, decision: allow}\nrules:\n- {id: stop, command_regex: rm, risk: high, decision: deny}\n")
	engine, _ := Load(path)
	report, err := RunTests(context.Background(), engine, []byte("version: 1\ncases:\n- name: deny remove\n  command: [rm, file]\n  expect: {risk: high, decision: deny}\n"))
	if err != nil || report.Passed != 1 || report.Failed != 0 {
		t.Fatalf("report=%+v err=%v", report, err)
	}
}

func BenchmarkPolicy100(b *testing.B)      { benchmarkPolicy(b, 100, false) }
func BenchmarkPolicy1000(b *testing.B)     { benchmarkPolicy(b, 1000, false) }
func BenchmarkSimulation100(b *testing.B)  { benchmarkPolicy(b, 100, true) }
func BenchmarkSimulation1000(b *testing.B) { benchmarkPolicy(b, 1000, true) }

func benchmarkPolicy(b *testing.B, count int, simulation bool) {
	var body strings.Builder
	body.WriteString("version: 1\ndefault: {risk: low, decision: allow}\nrules:\n")
	for index := 0; index < count; index++ {
		body.WriteString("- {id: r")
		body.WriteString(strings.Repeat("x", index%2))
		body.WriteString(fmtInt(index))
		body.WriteString(", command_regex: nomatch")
		body.WriteString(fmtInt(index))
		body.WriteString(", risk: medium, decision: warn}\n")
	}
	path := filepath.Join(b.TempDir(), "policy.yaml")
	_ = os.WriteFile(path, []byte(body.String()), 0o600)
	engine, err := Load(path)
	if err != nil {
		b.Fatal(err)
	}
	event := events.Event{Kind: events.KindCommand, Evidence: events.Evidence{Data: json.RawMessage(`{"command":["safe"]}`)}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if simulation {
			_, _ = engine.Simulate(context.Background(), event)
		} else {
			_, _, _ = engine.Evaluate(context.Background(), event)
		}
	}
}

func fmtInt(value int) string { return strconv.Itoa(value) }

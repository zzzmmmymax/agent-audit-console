package policy

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"gopkg.in/yaml.v3"
)

//go:embed default-tests.yaml
var DefaultTests []byte

type TestSuite struct {
	Version int        `yaml:"version" json:"version"`
	Cases   []TestCase `yaml:"cases" json:"cases"`
}

type TestCase struct {
	Name        string      `yaml:"name" json:"name"`
	Action      string      `yaml:"action" json:"action"`
	Kind        events.Kind `yaml:"kind,omitempty" json:"kind,omitempty"`
	Command     []string    `yaml:"command,omitempty" json:"command,omitempty"`
	Path        string      `yaml:"path,omitempty" json:"path,omitempty"`
	Target      string      `yaml:"target,omitempty" json:"target,omitempty"`
	Environment string      `yaml:"environment,omitempty" json:"environment,omitempty"`
	Expect      RuleResult  `yaml:"expect" json:"expect"`
}

type TestCaseResult struct {
	Name         string           `json:"name"`
	Passed       bool             `json:"passed"`
	Expected     RuleResult       `json:"expected"`
	Actual       RuleResult       `json:"actual"`
	MatchedRules []string         `json:"matched_rules"`
	Evaluation   EvaluationResult `json:"evaluation"`
}

type TestReport struct {
	Passed  int              `json:"passed"`
	Failed  int              `json:"failed"`
	Results []TestCaseResult `json:"results"`
}

func RunTests(ctx context.Context, engine *Engine, data []byte) (TestReport, error) {
	if len(data) > MaxPolicyBytes {
		return TestReport{}, fmt.Errorf("test file exceeds %d bytes", MaxPolicyBytes)
	}
	var suite TestSuite
	if err := yaml.Unmarshal(data, &suite); err != nil {
		return TestReport{}, fmt.Errorf("parse policy tests: %w", err)
	}
	if suite.Version != SchemaVersion {
		return TestReport{}, fmt.Errorf("unsupported policy test version %d", suite.Version)
	}
	if len(suite.Cases) > MaxRules {
		return TestReport{}, fmt.Errorf("test case count exceeds %d", MaxRules)
	}
	report := TestReport{Results: make([]TestCaseResult, 0, len(suite.Cases))}
	for index, test := range suite.Cases {
		if test.Name == "" {
			return TestReport{}, fmt.Errorf("case %d has no name", index)
		}
		decision, ok := NormalizeDecision(test.Expect.Decision)
		if !ok || !validRisk(test.Expect.Risk) {
			return TestReport{}, fmt.Errorf("case %s has invalid expectation", test.Name)
		}
		kind := test.Kind
		if kind == "" {
			kind = events.KindCommand
		}
		evidence, _ := json.Marshal(map[string]any{"command": test.Command, "path": test.Path, "target": test.Target, "environment": test.Environment})
		evaluation, err := engine.Simulate(ctx, events.Event{Kind: kind, Intent: test.Action, Evidence: events.Evidence{Data: evidence}})
		if err != nil {
			return TestReport{}, err
		}
		actual := RuleResult{Risk: evaluation.RiskLevel, Decision: evaluation.Decision}
		passed := actual == (RuleResult{Risk: test.Expect.Risk, Decision: decision})
		result := TestCaseResult{Name: test.Name, Passed: passed, Expected: RuleResult{Risk: test.Expect.Risk, Decision: decision}, Actual: actual, MatchedRules: ruleIDs(evaluation.MatchedRules), Evaluation: evaluation}
		report.Results = append(report.Results, result)
		if passed {
			report.Passed++
		} else {
			report.Failed++
		}
	}
	return report, nil
}

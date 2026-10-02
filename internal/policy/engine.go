package policy

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"gopkg.in/yaml.v3"
)

//go:embed default-rules.yaml
var DefaultRules []byte

type Config struct {
	Version int        `yaml:"version"`
	Default RuleResult `yaml:"default"`
	Rules   []Rule     `yaml:"rules"`
}
type RuleResult struct {
	Risk     string `yaml:"risk"`
	Decision string `yaml:"decision"`
}
type Rule struct {
	ID           string      `yaml:"id"`
	Description  string      `yaml:"description"`
	Kind         events.Kind `yaml:"kind,omitempty"`
	CommandRegex string      `yaml:"command_regex,omitempty"`
	Risk         string      `yaml:"risk"`
	Decision     string      `yaml:"decision"`
	expression   *regexp.Regexp
}
type Engine struct {
	config  Config
	sources []string
}

func EnsureDefault(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := file.Write(DefaultRules); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func Load(path string) (*Engine, error) {
	return LoadLayered(path)
}

// LoadLayered evaluates team rules before local rules, preventing a local rule
// from silently weakening a matching shared rule.
func LoadLayered(localPath string, teamPaths ...string) (*Engine, error) {
	local, err := loadConfig(localPath)
	if err != nil {
		return nil, err
	}
	merged := local
	merged.Rules = nil
	sources := []string{}
	for _, path := range teamPaths {
		if path == "" {
			continue
		}
		team, err := loadConfig(path)
		if err != nil {
			return nil, fmt.Errorf("team policy %s: %w", path, err)
		}
		merged.Rules = append(merged.Rules, team.Rules...)
		merged.Default = stricterResult(merged.Default, team.Default)
		sources = append(sources, path)
	}
	merged.Rules = append(merged.Rules, local.Rules...)
	sources = append(sources, localPath)
	return &Engine{config: merged, sources: sources}, nil
}

func stricterResult(left, right RuleResult) RuleResult {
	result := left
	if riskRank(right.Risk) > riskRank(result.Risk) {
		result.Risk = right.Risk
	}
	if decisionRank(right.Decision) > decisionRank(result.Decision) {
		result.Decision = right.Decision
	}
	return result
}

func riskRank(value string) int {
	switch value {
	case events.RiskHigh:
		return 3
	case events.RiskMedium:
		return 2
	case events.RiskLow:
		return 1
	default:
		return 0
	}
}

func decisionRank(value string) int {
	switch value {
	case events.PolicyRejected:
		return 3
	case events.PolicyPending:
		return 2
	case events.PolicyAllowed:
		return 1
	default:
		return 0
	}
}

func loadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("parse policy: %w", err)
	}
	if config.Version != 1 {
		return Config{}, fmt.Errorf("unsupported policy version %d", config.Version)
	}
	if !validRisk(config.Default.Risk) || !validDecision(config.Default.Decision) {
		return Config{}, fmt.Errorf("invalid default policy result")
	}
	seen := map[string]struct{}{}
	for i := range config.Rules {
		if config.Rules[i].ID == "" {
			return Config{}, fmt.Errorf("rule %d has no id", i)
		}
		if _, exists := seen[config.Rules[i].ID]; exists {
			return Config{}, fmt.Errorf("duplicate rule id %s", config.Rules[i].ID)
		}
		seen[config.Rules[i].ID] = struct{}{}
		if !validRisk(config.Rules[i].Risk) || !validDecision(config.Rules[i].Decision) {
			return Config{}, fmt.Errorf("rule %s has invalid risk or decision", config.Rules[i].ID)
		}
		if config.Rules[i].CommandRegex != "" {
			compiled, err := regexp.Compile(config.Rules[i].CommandRegex)
			if err != nil {
				return Config{}, fmt.Errorf("rule %s: %w", config.Rules[i].ID, err)
			}
			config.Rules[i].expression = compiled
		}
	}
	return config, nil
}

func (e *Engine) Sources() []string { return append([]string(nil), e.sources...) }

func (e *Engine) Ready() error {
	for _, source := range e.sources {
		if _, err := os.Stat(source); err != nil {
			return err
		}
	}
	return nil
}

func validRisk(value string) bool {
	return value == events.RiskLow || value == events.RiskMedium || value == events.RiskHigh
}
func validDecision(value string) bool {
	return value == events.PolicyAllowed || value == events.PolicyPending || value == events.PolicyRejected
}

func (e *Engine) Evaluate(_ context.Context, event events.Event) (events.Risk, events.PolicyDecision, error) {
	command := commandText(event)
	for _, rule := range e.config.Rules {
		if rule.Kind != "" && rule.Kind != event.Kind {
			continue
		}
		if rule.expression != nil && !rule.expression.MatchString(command) {
			continue
		}
		return events.Risk{Level: rule.Risk, Reasons: []string{rule.Description}}, events.PolicyDecision{Status: rule.Decision, RuleID: rule.ID, Reason: rule.Description}, nil
	}
	return events.Risk{Level: e.config.Default.Risk}, events.PolicyDecision{Status: e.config.Default.Decision}, nil
}
func commandText(event events.Event) string {
	var value struct {
		Command []string `json:"command"`
	}
	if json.Unmarshal(event.Evidence.Data, &value) != nil {
		return ""
	}
	return strings.Join(value.Command, " ")
}

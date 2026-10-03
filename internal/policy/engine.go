package policy

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"gopkg.in/yaml.v3"
)

//go:embed default-rules.yaml
var DefaultRules []byte

const (
	SchemaVersion    = 1
	MaxPolicyBytes   = 1 << 20
	MaxRules         = 10000
	MaxStringLength  = 4096
	MaxPatternLength = 2048

	DecisionAllow    = "allow"
	DecisionWarn     = "warn"
	DecisionApproval = "require_approval"
	DecisionDeny     = "deny"

	SourceBuiltin = "builtin"
	SourceProject = "project"
	SourceTeam    = "team"
	SourceUser    = "user"
)

type Config struct {
	Version int        `yaml:"version" json:"version"`
	Default RuleResult `yaml:"default" json:"default"`
	Rules   []Rule     `yaml:"rules" json:"rules"`
}

type RuleResult struct {
	Risk     string `yaml:"risk" json:"risk"`
	Decision string `yaml:"decision" json:"decision"`
}

type Rule struct {
	ID           string      `yaml:"id" json:"id"`
	Name         string      `yaml:"name,omitempty" json:"name,omitempty"`
	Description  string      `yaml:"description,omitempty" json:"description,omitempty"`
	Kind         events.Kind `yaml:"kind,omitempty" json:"kind,omitempty"`
	CommandRegex string      `yaml:"command_regex,omitempty" json:"command_regex,omitempty"`
	Risk         string      `yaml:"risk" json:"risk"`
	Decision     string      `yaml:"decision" json:"decision"`
	Priority     int         `yaml:"priority,omitempty" json:"priority,omitempty"`
	Source       string      `yaml:"-" json:"source"`
	PolicyFile   string      `yaml:"-" json:"policy_file,omitempty"`
	expression   *regexp.Regexp
}

type RuleMatch struct {
	RuleID     string         `json:"rule_id"`
	RuleName   string         `json:"rule_name,omitempty"`
	Source     string         `json:"source"`
	PolicyFile string         `json:"policy_file,omitempty"`
	Priority   int            `json:"priority"`
	RiskLevel  string         `json:"risk_level"`
	Decision   string         `json:"decision"`
	Reason     string         `json:"reason,omitempty"`
	Conditions map[string]any `json:"conditions,omitempty"`
	Overridden bool           `json:"overridden"`
}

type TraceStep struct {
	RuleID   string `json:"rule_id,omitempty"`
	Source   string `json:"source,omitempty"`
	Matched  bool   `json:"matched,omitempty"`
	Decision string `json:"decision,omitempty"`
	Risk     string `json:"risk_level,omitempty"`
	Message  string `json:"message"`
}

type EvaluationResult struct {
	Decision             string         `json:"decision"`
	RiskLevel            string         `json:"risk_level"`
	MatchedRules         []RuleMatch    `json:"matched_rules"`
	EffectiveRule        *RuleMatch     `json:"effective_rule,omitempty"`
	Reason               string         `json:"reason"`
	PolicySource         string         `json:"policy_source"`
	PolicyFile           string         `json:"policy_file,omitempty"`
	RuleID               string         `json:"rule_id,omitempty"`
	RuleName             string         `json:"rule_name,omitempty"`
	RulePriority         int            `json:"rule_priority,omitempty"`
	Conditions           map[string]any `json:"conditions,omitempty"`
	OverriddenRules      []RuleMatch    `json:"overridden_rules"`
	RequiresApproval     bool           `json:"requires_approval"`
	ReversibilityContext string         `json:"reversibility_context,omitempty"`
	Trace                []TraceStep    `json:"trace"`
	PolicyFingerprint    string         `json:"policy_fingerprint"`
	PolicySchemaVersion  int            `json:"policy_schema_version"`
}

type ValidationIssue struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	RuleID   string `json:"rule_id,omitempty"`
	Message  string `json:"message"`
}

type ValidationReport struct {
	Valid       bool              `json:"valid"`
	Errors      []ValidationIssue `json:"errors"`
	Warnings    []ValidationIssue `json:"warnings"`
	RuleCount   int               `json:"rule_count"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	Config      *Config           `json:"-"`
}

type Inspection struct {
	SchemaVersion int        `json:"schema_version"`
	Fingerprint   string     `json:"fingerprint"`
	Sources       []string   `json:"sources"`
	RuleCount     int        `json:"rule_count"`
	Default       RuleResult `json:"default"`
	Rules         []Rule     `json:"rules"`
	Health        string     `json:"health"`
}

type Engine struct {
	mu          sync.RWMutex
	config      Config
	sources     []string
	fingerprint string
	health      string
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
	defer file.Close()
	_, err = file.Write(DefaultRules)
	return err
}

func Load(path string) (*Engine, error) { return LoadLayered(path) }

// LoadLayered merges configured team policy with project policy. A stricter
// matching decision wins; risk is assessed separately and also takes the maximum.
func LoadLayered(localPath string, teamPaths ...string) (*Engine, error) {
	merged, sources, err := loadLayered(localPath, teamPaths...)
	if err != nil {
		return nil, err
	}
	fingerprint, err := Fingerprint(merged)
	if err != nil {
		return nil, err
	}
	return &Engine{config: merged, sources: sources, fingerprint: fingerprint, health: "PASS — policy valid"}, nil
}

func loadLayered(localPath string, teamPaths ...string) (Config, []string, error) {
	local, err := loadConfig(localPath, SourceProject)
	if err != nil {
		return Config{}, nil, err
	}
	merged := local
	merged.Rules = nil
	sources := make([]string, 0, len(teamPaths)+1)
	for _, path := range teamPaths {
		if path == "" {
			continue
		}
		team, loadErr := loadConfig(path, SourceTeam)
		if loadErr != nil {
			return Config{}, nil, fmt.Errorf("team policy %s: %w", path, loadErr)
		}
		merged.Rules = append(merged.Rules, team.Rules...)
		merged.Default = stricterResult(merged.Default, team.Default)
		sources = append(sources, path)
	}
	merged.Rules = append(merged.Rules, local.Rules...)
	sources = append(sources, localPath)
	sortRules(merged.Rules)
	return merged, sources, nil
}

func NormalizeDecision(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case DecisionAllow, events.PolicyAllowed, events.PolicyApproved:
		return DecisionAllow, true
	case DecisionWarn, "warning":
		return DecisionWarn, true
	case DecisionApproval, events.PolicyPending, "confirm":
		return DecisionApproval, true
	case DecisionDeny, events.PolicyRejected, "blocked":
		return DecisionDeny, true
	default:
		return "", false
	}
}

func EventDecision(value string) string {
	switch value {
	case DecisionApproval:
		return events.PolicyPending
	case DecisionDeny:
		return events.PolicyRejected
	default:
		return events.PolicyAllowed
	}
}

func DecisionRank(value string) int {
	canonical, _ := NormalizeDecision(value)
	switch canonical {
	case DecisionDeny:
		return 4
	case DecisionApproval:
		return 3
	case DecisionWarn:
		return 2
	case DecisionAllow:
		return 1
	default:
		return 0
	}
}

func RiskRank(value string) int {
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

func stricterResult(left, right RuleResult) RuleResult {
	result := left
	if RiskRank(right.Risk) > RiskRank(result.Risk) {
		result.Risk = right.Risk
	}
	if DecisionRank(right.Decision) > DecisionRank(result.Decision) {
		result.Decision, _ = NormalizeDecision(right.Decision)
	} else {
		result.Decision, _ = NormalizeDecision(result.Decision)
	}
	return result
}

func ValidateDocument(data []byte) ValidationReport {
	report := ValidationReport{Valid: false, Errors: []ValidationIssue{}, Warnings: []ValidationIssue{}}
	if len(data) > MaxPolicyBytes {
		report.Errors = append(report.Errors, issue("error", "policy_too_large", "", fmt.Sprintf("policy exceeds %d bytes", MaxPolicyBytes)))
		return report
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		report.Errors = append(report.Errors, issue("error", "invalid_yaml", "", err.Error()))
		return report
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		message := "policy must contain exactly one YAML document"
		if err != nil {
			message = err.Error()
		}
		report.Errors = append(report.Errors, issue("error", "multiple_documents", "", message))
		return report
	}
	report.RuleCount = len(config.Rules)
	if config.Version != SchemaVersion {
		report.Errors = append(report.Errors, issue("error", "unsupported_version", "", fmt.Sprintf("unsupported policy version %d", config.Version)))
	}
	if len(config.Rules) > MaxRules {
		report.Errors = append(report.Errors, issue("error", "too_many_rules", "", fmt.Sprintf("rule count exceeds %d", MaxRules)))
	}
	if !validRisk(config.Default.Risk) {
		report.Errors = append(report.Errors, issue("error", "invalid_risk", "", "default risk must be low, medium, or high"))
	}
	if normalized, ok := NormalizeDecision(config.Default.Decision); !ok {
		report.Errors = append(report.Errors, issue("error", "invalid_decision", "", "default decision is invalid"))
	} else {
		config.Default.Decision = normalized
	}
	seen := map[string]struct{}{}
	matcher := map[string]Rule{}
	for index := range config.Rules {
		rule := &config.Rules[index]
		rule.ID = strings.TrimSpace(rule.ID)
		if rule.ID == "" {
			report.Errors = append(report.Errors, issue("error", "missing_rule_id", "", fmt.Sprintf("rule %d has no id", index)))
			continue
		}
		if len(rule.ID) > MaxStringLength || len(rule.Name) > MaxStringLength || len(rule.Description) > MaxStringLength {
			report.Errors = append(report.Errors, issue("error", "string_too_long", rule.ID, "rule string exceeds limit"))
		}
		if _, exists := seen[rule.ID]; exists {
			report.Errors = append(report.Errors, issue("error", "duplicate_rule_id", rule.ID, "duplicate rule id"))
		}
		seen[rule.ID] = struct{}{}
		if !validRisk(rule.Risk) {
			report.Errors = append(report.Errors, issue("error", "invalid_risk", rule.ID, "risk must be low, medium, or high"))
		}
		if normalized, ok := NormalizeDecision(rule.Decision); !ok {
			report.Errors = append(report.Errors, issue("error", "invalid_decision", rule.ID, "decision is invalid"))
		} else {
			rule.Decision = normalized
		}
		if rule.Kind == "" && rule.CommandRegex == "" {
			report.Warnings = append(report.Warnings, issue("warning", "broad_matcher", rule.ID, "rule has no matcher and matches every action"))
		}
		if len(rule.CommandRegex) > MaxPatternLength {
			report.Errors = append(report.Errors, issue("error", "pattern_too_long", rule.ID, "command_regex exceeds limit"))
		} else if rule.CommandRegex != "" {
			compiled, err := regexp.Compile(rule.CommandRegex)
			if err != nil {
				report.Errors = append(report.Errors, issue("error", "invalid_matcher", rule.ID, err.Error()))
			} else {
				rule.expression = compiled
			}
		}
		key := string(rule.Kind) + "\x00" + rule.CommandRegex
		if prior, exists := matcher[key]; exists && prior.Decision != rule.Decision {
			report.Warnings = append(report.Warnings, issue("warning", "overlapping_rules", rule.ID, fmt.Sprintf("same matcher as %s; strictest decision wins", prior.ID)))
		} else {
			matcher[key] = *rule
		}
	}
	if len(report.Errors) == 0 {
		report.Valid = true
		report.Config = &config
		report.Fingerprint, _ = Fingerprint(config)
	}
	return report
}

func issue(severity, code, ruleID, message string) ValidationIssue {
	return ValidationIssue{Severity: severity, Code: code, RuleID: ruleID, Message: message}
}

func loadConfig(path, source string) (Config, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Config{}, err
	}
	if info.Size() > MaxPolicyBytes {
		return Config{}, fmt.Errorf("policy exceeds %d bytes", MaxPolicyBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	report := ValidateDocument(data)
	if !report.Valid {
		return Config{}, errors.New(report.Errors[0].Message)
	}
	config := *report.Config
	for index := range config.Rules {
		config.Rules[index].Source = source
		config.Rules[index].PolicyFile = filepath.Base(path)
	}
	return config, nil
}

func Fingerprint(config Config) (string, error) {
	stable := config
	stable.Rules = append([]Rule(nil), config.Rules...)
	for index := range stable.Rules {
		stable.Rules[index].expression = nil
		stable.Rules[index].PolicyFile = ""
	}
	sort.SliceStable(stable.Rules, func(i, j int) bool {
		if stable.Rules[i].ID != stable.Rules[j].ID {
			return stable.Rules[i].ID < stable.Rules[j].ID
		}
		return stable.Rules[i].Source < stable.Rules[j].Source
	})
	data, err := json.Marshal(stable)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func sortRules(rules []Rule) {
	sort.SliceStable(rules, func(i, j int) bool {
		if rules[i].Priority != rules[j].Priority {
			return rules[i].Priority > rules[j].Priority
		}
		if rules[i].Source != rules[j].Source {
			return sourceRank(rules[i].Source) > sourceRank(rules[j].Source)
		}
		return rules[i].ID < rules[j].ID
	})
}

func sourceRank(source string) int {
	switch source {
	case SourceTeam:
		return 4
	case SourceUser:
		return 3
	case SourceProject:
		return 2
	case SourceBuiltin:
		return 1
	default:
		return 0
	}
}

func (e *Engine) Sources() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]string(nil), e.sources...)
}

func (e *Engine) Fingerprint() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.fingerprint
}

func (e *Engine) Health() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.health
}

func (e *Engine) Ready() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.sources) == 0 || e.fingerprint == "" {
		return errors.New("no valid policy available")
	}
	return nil
}

// Reload is fail-safe: an invalid replacement leaves the last-known-good
// configuration active and surfaces a health warning.
func (e *Engine) Reload() error {
	e.mu.RLock()
	sources := append([]string(nil), e.sources...)
	e.mu.RUnlock()
	if len(sources) == 0 {
		return errors.New("no policy sources")
	}
	config, nextSources, err := loadLayered(sources[len(sources)-1], sources[:len(sources)-1]...)
	if err != nil {
		e.mu.Lock()
		e.health = "WARN — current policy invalid, using last-known-good: " + err.Error()
		e.mu.Unlock()
		return err
	}
	fingerprint, err := Fingerprint(config)
	if err != nil {
		return err
	}
	e.mu.Lock()
	e.config, e.sources, e.fingerprint = config, nextSources, fingerprint
	e.health = "PASS — policy valid"
	e.mu.Unlock()
	return nil
}

func (e *Engine) Inspect() Inspection {
	e.mu.RLock()
	defer e.mu.RUnlock()
	rules := append([]Rule(nil), e.config.Rules...)
	sources := make([]string, len(e.sources))
	for index, source := range e.sources {
		sources[index] = filepath.Base(source)
	}
	return Inspection{SchemaVersion: e.config.Version, Fingerprint: e.fingerprint, Sources: sources, RuleCount: len(rules), Default: e.config.Default, Rules: rules, Health: e.health}
}

func validRisk(value string) bool {
	return value == events.RiskLow || value == events.RiskMedium || value == events.RiskHigh
}

// Simulate is purely in-memory and never writes audit events, approvals, files,
// rollback state, or telemetry.
func (e *Engine) Simulate(_ context.Context, event events.Event) (EvaluationResult, error) {
	e.mu.RLock()
	config := e.config
	fingerprint := e.fingerprint
	e.mu.RUnlock()
	command := commandText(event)
	result := EvaluationResult{
		Decision:             config.Default.Decision,
		RiskLevel:            config.Default.Risk,
		MatchedRules:         []RuleMatch{},
		OverriddenRules:      []RuleMatch{},
		Trace:                []TraceStep{{Message: fmt.Sprintf("default result is %s/%s", config.Default.Risk, config.Default.Decision), Decision: config.Default.Decision, Risk: config.Default.Risk}},
		Reason:               "No rule matched; default policy applies.",
		PolicySource:         SourceProject,
		PolicyFingerprint:    fingerprint,
		PolicySchemaVersion:  config.Version,
		ReversibilityContext: event.Reversibility.Status,
	}
	var effectiveIndex = -1
	for _, rule := range config.Rules {
		matched := true
		message := "rule matched"
		if rule.Kind != "" && rule.Kind != event.Kind {
			matched, message = false, "kind did not match"
		} else if rule.expression != nil && !rule.expression.MatchString(command) {
			matched, message = false, "command_regex did not match"
		}
		result.Trace = append(result.Trace, TraceStep{RuleID: rule.ID, Source: rule.Source, Matched: matched, Decision: rule.Decision, Risk: rule.Risk, Message: message})
		if !matched {
			continue
		}
		conditions := map[string]any{}
		if rule.Kind != "" {
			conditions["kind"] = rule.Kind
		}
		if rule.CommandRegex != "" {
			conditions["command_regex"] = rule.CommandRegex
		}
		match := RuleMatch{RuleID: rule.ID, RuleName: rule.Name, Source: rule.Source, PolicyFile: rule.PolicyFile, Priority: rule.Priority, RiskLevel: rule.Risk, Decision: rule.Decision, Reason: rule.Description, Conditions: conditions}
		result.MatchedRules = append(result.MatchedRules, match)
		if RiskRank(rule.Risk) > RiskRank(result.RiskLevel) {
			result.RiskLevel = rule.Risk
		}
		if effectiveIndex == -1 || DecisionRank(rule.Decision) > DecisionRank(result.MatchedRules[effectiveIndex].Decision) {
			effectiveIndex = len(result.MatchedRules) - 1
		}
	}
	if effectiveIndex >= 0 {
		for index := range result.MatchedRules {
			result.MatchedRules[index].Overridden = index != effectiveIndex
			if index != effectiveIndex {
				result.OverriddenRules = append(result.OverriddenRules, result.MatchedRules[index])
			}
		}
		effective := result.MatchedRules[effectiveIndex]
		result.EffectiveRule = &effective
		result.Decision = effective.Decision
		result.Reason = effective.Reason
		if result.Reason == "" {
			result.Reason = fmt.Sprintf("Rule %s selected the strictest matching decision.", effective.RuleID)
		}
		result.PolicySource, result.PolicyFile = effective.Source, effective.PolicyFile
		result.RuleID, result.RuleName, result.RulePriority = effective.RuleID, effective.RuleName, effective.Priority
		result.Conditions = effective.Conditions
	}
	result.RequiresApproval = result.Decision == DecisionApproval
	result.Trace = append(result.Trace, TraceStep{RuleID: result.RuleID, Source: result.PolicySource, Matched: effectiveIndex >= 0, Decision: result.Decision, Risk: result.RiskLevel, Message: "final strictest decision and highest risk selected"})
	return result, nil
}

func (e *Engine) Evaluate(ctx context.Context, event events.Event) (events.Risk, events.PolicyDecision, error) {
	result, err := e.Simulate(ctx, event)
	if err != nil {
		return events.Risk{}, events.PolicyDecision{}, err
	}
	reasons := []string{}
	if result.Reason != "" {
		reasons = append(reasons, result.Reason)
	}
	return events.Risk{Level: result.RiskLevel, Reasons: reasons}, events.PolicyDecision{
		Status: EventDecision(result.Decision), RuleID: result.RuleID, Reason: result.Reason,
		Decision: result.Decision, PolicySource: result.PolicySource, PolicyFile: result.PolicyFile,
		RuleName: result.RuleName, RulePriority: result.RulePriority, MatchedRules: ruleIDs(result.MatchedRules),
		PolicyFingerprint: result.PolicyFingerprint, PolicySchemaVersion: result.PolicySchemaVersion,
		RequiresApproval: result.RequiresApproval,
	}, nil
}

func ruleIDs(matches []RuleMatch) []string {
	ids := make([]string, 0, len(matches))
	for _, match := range matches {
		ids = append(ids, match.RuleID)
	}
	return ids
}

func commandText(event events.Event) string {
	var value struct {
		Command []string `json:"command"`
		Path    string   `json:"path"`
		Target  string   `json:"target"`
	}
	if json.Unmarshal(event.Evidence.Data, &value) != nil {
		return ""
	}
	return strings.TrimSpace(strings.Join(value.Command, " ") + " " + value.Path + " " + value.Target)
}

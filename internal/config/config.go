// Package config resolves runtime configuration using the documented priority:
// explicit CLI override, environment, YAML config file, then defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const DefaultSyncTokenEnv = "AGENT_AUDIT_SYNC_TOKEN"

type Runtime struct {
	ConfigFile    string   `yaml:"-"`
	DataDir       string   `yaml:"data_dir"`
	DatabasePath  string   `yaml:"database_path"`
	SnapshotDir   string   `yaml:"snapshot_dir"`
	PolicyFile    string   `yaml:"policy_file"`
	ListenAddress string   `yaml:"listen_address"`
	AccessFile    string   `yaml:"access_file"`
	TeamPolicies  []string `yaml:"team_policies"`
	SyncEndpoint  string   `yaml:"sync_endpoint"`
	SyncTokenEnv  string   `yaml:"sync_token_env"`
	RetentionDays int      `yaml:"retention_days"`
}

func Defaults() Runtime {
	base, err := os.UserConfigDir()
	if err != nil {
		base = "."
	}
	data := filepath.Join(base, "agent-audit-console")
	return Runtime{DataDir: data, ListenAddress: "127.0.0.1:8777", SyncTokenEnv: DefaultSyncTokenEnv}
}

func Load(path string) (Runtime, error) {
	value := Defaults()
	explicit := strings.TrimSpace(path)
	if explicit == "" {
		explicit = strings.TrimSpace(os.Getenv("AGENT_AUDIT_CONFIG"))
	}
	if explicit != "" {
		data, err := os.ReadFile(explicit)
		if err != nil {
			return Runtime{}, fmt.Errorf("read config %s: %w", explicit, err)
		}
		if err := yaml.Unmarshal(data, &value); err != nil {
			return Runtime{}, fmt.Errorf("parse config %s: %w", explicit, err)
		}
		value.ConfigFile = explicit
	}
	applyStringEnv(&value.DataDir, "AGENT_AUDIT_HOME")
	applyStringEnv(&value.DatabasePath, "AGENT_AUDIT_DATABASE")
	applyStringEnv(&value.SnapshotDir, "AGENT_AUDIT_SNAPSHOT_DIR")
	applyStringEnv(&value.PolicyFile, "AGENT_AUDIT_POLICY_FILE")
	applyStringEnv(&value.ListenAddress, "AGENT_AUDIT_LISTEN_ADDRESS")
	applyStringEnv(&value.AccessFile, "AGENT_AUDIT_ACCESS_FILE")
	applyStringEnv(&value.SyncEndpoint, "AGENT_AUDIT_SYNC_ENDPOINT")
	applyStringEnv(&value.SyncTokenEnv, "AGENT_AUDIT_SYNC_TOKEN_ENV")
	if raw, ok := os.LookupEnv("AGENT_AUDIT_TEAM_POLICY"); ok {
		value.TeamPolicies = nonEmpty(filepath.SplitList(raw))
	}
	if raw, ok := os.LookupEnv("AGENT_AUDIT_RETENTION_DAYS"); ok {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return Runtime{}, fmt.Errorf("AGENT_AUDIT_RETENTION_DAYS: %w", err)
		}
		value.RetentionDays = parsed
	}
	return value.Finalize()
}

func (c Runtime) Finalize() (Runtime, error) {
	if strings.TrimSpace(c.DataDir) == "" {
		return Runtime{}, errors.New("data directory cannot be empty")
	}
	if c.RetentionDays < 0 {
		return Runtime{}, errors.New("retention_days cannot be negative")
	}
	absolute, err := filepath.Abs(c.DataDir)
	if err != nil {
		return Runtime{}, fmt.Errorf("resolve data directory: %w", err)
	}
	c.DataDir = filepath.Clean(absolute)
	if c.DatabasePath == "" {
		c.DatabasePath = filepath.Join(c.DataDir, "audit.db")
	}
	if c.SnapshotDir == "" {
		c.SnapshotDir = filepath.Join(c.DataDir, "snapshots")
	}
	if c.PolicyFile == "" {
		c.PolicyFile = filepath.Join(c.DataDir, "policy.yaml")
	}
	if c.AccessFile == "" {
		c.AccessFile = filepath.Join(c.DataDir, "access.yaml")
	}
	if c.ListenAddress == "" {
		c.ListenAddress = "127.0.0.1:8777"
	}
	if c.SyncTokenEnv == "" {
		c.SyncTokenEnv = DefaultSyncTokenEnv
	}
	for _, pointer := range []*string{&c.DatabasePath, &c.SnapshotDir, &c.PolicyFile, &c.AccessFile} {
		if !filepath.IsAbs(*pointer) {
			*pointer = filepath.Join(c.DataDir, *pointer)
		}
		*pointer = filepath.Clean(*pointer)
	}
	c.TeamPolicies = nonEmpty(c.TeamPolicies)
	return c, nil
}

func (c Runtime) WithDataDir(path string) (Runtime, error) {
	if path == "" {
		return c.Finalize()
	}
	c.DataDir, c.DatabasePath, c.SnapshotDir, c.PolicyFile, c.AccessFile = path, "", "", "", ""
	return c.Finalize()
}

func applyStringEnv(target *string, name string) {
	if value, ok := os.LookupEnv(name); ok {
		*target = value
	}
}

func nonEmpty(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, filepath.Clean(value))
		}
	}
	return result
}

package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Agent string

const (
	Codex      Agent = "codex"
	ClaudeCode Agent = "claude-code"
	Cursor     Agent = "cursor"
)

type Plan struct {
	Agent      Agent    `json:"agent"`
	ConfigPath string   `json:"config_path"`
	Format     string   `json:"format"`
	ServerName string   `json:"server_name"`
	Command    string   `json:"command"`
	Args       []string `json:"args,omitempty"`
	Exists     bool     `json:"exists"`
}

type Result struct {
	Plan       Plan   `json:"plan"`
	Applied    bool   `json:"applied"`
	BackupPath string `json:"backup_path,omitempty"`
}

func Locate(agent Agent, override string) (string, error) {
	if override != "" {
		return filepath.Abs(override)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	switch agent {
	case Codex:
		return filepath.Join(home, ".codex", "config.toml"), nil
	case ClaudeCode:
		return filepath.Join(home, ".claude.json"), nil
	case Cursor:
		return filepath.Join(home, ".cursor", "mcp.json"), nil
	default:
		return "", fmt.Errorf("unsupported integration %q", agent)
	}
}

func BuildPlan(agent Agent, configPath, command string, args []string) (Plan, error) {
	path, err := Locate(agent, configPath)
	if err != nil {
		return Plan{}, err
	}
	if command == "" {
		if found, lookErr := exec.LookPath(executableName("agent-audit-mcp")); lookErr == nil {
			command, err = filepath.Abs(found)
		} else {
			command, err = os.Executable()
			if err == nil {
				command = filepath.Join(filepath.Dir(command), executableName("agent-audit-mcp"))
			}
		}
		if err != nil {
			return Plan{}, err
		}
	}
	_, statErr := os.Stat(path)
	if statErr != nil && !os.IsNotExist(statErr) {
		return Plan{}, statErr
	}
	format := "json"
	if agent == Codex {
		format = "toml"
	}
	return Plan{Agent: agent, ConfigPath: path, Format: format, ServerName: "agent-audit", Command: command, Args: args, Exists: statErr == nil}, nil
}

func executableName(name string) string {
	if strings.EqualFold(filepath.Ext(os.Args[0]), ".exe") {
		return name + ".exe"
	}
	return name
}

func Preview(plan Plan) string {
	return fmt.Sprintf("Will add MCP server:\n  name: %s\n  agent: %s\n  config: %s\n  command: %s\n", plan.ServerName, plan.Agent, plan.ConfigPath, plan.Command)
}

func Apply(plan Plan) (Result, error) {
	if info, err := os.Stat(plan.Command); err != nil || info.IsDir() {
		return Result{}, fmt.Errorf("MCP command is not an executable file: %s", plan.Command)
	}
	current, err := os.ReadFile(plan.ConfigPath)
	if err != nil && !os.IsNotExist(err) {
		return Result{}, err
	}
	var merged []byte
	if plan.Format == "toml" {
		merged, err = mergeTOML(current, plan)
	} else {
		merged, err = mergeJSON(current, plan)
	}
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(plan.ConfigPath), 0700); err != nil {
		return Result{}, err
	}
	result := Result{Plan: plan, Applied: true}
	if len(current) > 0 {
		result.BackupPath, err = backup(plan.ConfigPath, current)
		if err != nil {
			return Result{}, err
		}
	}
	if err = atomicWrite(plan.ConfigPath, merged); err != nil {
		return Result{}, err
	}
	pruneBackups(plan.ConfigPath, 3)
	return result, nil
}

func mergeJSON(current []byte, plan Plan) ([]byte, error) {
	root := map[string]any{}
	if len(bytes.TrimSpace(current)) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(current))
		decoder.UseNumber()
		if err := decoder.Decode(&root); err != nil {
			return nil, fmt.Errorf("unrecognized or malformed JSON config: %w", err)
		}
	}
	servers, ok := root["mcpServers"].(map[string]any)
	if !ok {
		if root["mcpServers"] != nil {
			return nil, errors.New("mcpServers must be an object")
		}
		servers = map[string]any{}
		root["mcpServers"] = servers
	}
	servers[plan.ServerName] = map[string]any{"command": plan.Command, "args": plan.Args}
	encoded, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(encoded, '\n'), nil
}

func mergeTOML(current []byte, plan Plan) ([]byte, error) {
	text := strings.ReplaceAll(string(current), "\r\n", "\n")
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed, "]") {
			return nil, errors.New("unrecognized or malformed TOML section")
		}
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "[") && !strings.Contains(trimmed, "=") {
			return nil, errors.New("unrecognized or malformed TOML config")
		}
	}
	header := "[mcp_servers." + plan.ServerName + "]"
	lines := strings.Split(text, "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if t == header {
			start = i
			continue
		}
		if start >= 0 && i > start && strings.HasPrefix(t, "[") {
			end = i
			break
		}
	}
	block := []string{header, "command = " + strconv.Quote(plan.Command)}
	if len(plan.Args) > 0 {
		quoted := make([]string, len(plan.Args))
		for i, arg := range plan.Args {
			quoted[i] = strconv.Quote(arg)
		}
		block = append(block, "args = ["+strings.Join(quoted, ", ")+"]")
	}
	if start >= 0 {
		lines = append(append(append([]string{}, lines[:start]...), block...), lines[end:]...)
	} else {
		if strings.TrimSpace(text) != "" {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"), nil
}

func backup(path string, data []byte) (string, error) {
	name := path + ".bak-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	return name, os.WriteFile(name, data, 0600)
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".agent-audit-config-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err = tmp.Chmod(0600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if _, err = os.Stat(path); os.IsNotExist(err) {
		return os.Rename(name, path)
	} else if err != nil {
		return err
	}
	old := path + ".replace-old"
	_ = os.Remove(old)
	if err = os.Rename(path, old); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		_ = os.Rename(old, path)
		return err
	}
	return os.Remove(old)
}

func pruneBackups(path string, keep int) {
	items, _ := filepath.Glob(path + ".bak-*")
	sort.Strings(items)
	for len(items) > keep {
		_ = os.Remove(items[0])
		items = items[1:]
	}
}

func Check(agent Agent, path string) (string, string) {
	resolved, err := Locate(agent, path)
	if err != nil {
		return "FAIL", err.Error()
	}
	data, err := os.ReadFile(resolved)
	if os.IsNotExist(err) {
		return "WARN", "not configured"
	}
	if err != nil {
		return "FAIL", err.Error()
	}
	plan := Plan{Agent: agent, ConfigPath: resolved, Format: "json", ServerName: "agent-audit"}
	if agent == Codex {
		plan.Format = "toml"
		_, err = mergeTOML(data, plan)
		if err == nil && !strings.Contains(string(data), "[mcp_servers.agent-audit]") {
			return "WARN", "not configured"
		}
	} else {
		var root map[string]any
		err = json.Unmarshal(data, &root)
		if err == nil {
			servers, _ := root["mcpServers"].(map[string]any)
			if servers["agent-audit"] == nil {
				return "WARN", "not configured"
			}
		}
	}
	if err != nil {
		return "FAIL", err.Error()
	}
	return "PASS", "Agent Audit MCP configured"
}

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-audit-console/agent-audit-console/internal/audit"
	"github.com/agent-audit-console/agent-audit-console/internal/auth"
	"github.com/agent-audit-console/agent-audit-console/internal/buildinfo"
	runtimeconfig "github.com/agent-audit-console/agent-audit-console/internal/config"
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"github.com/agent-audit-console/agent-audit-console/internal/integrations"
	"github.com/agent-audit-console/agent-audit-console/internal/policy"
	"github.com/agent-audit-console/agent-audit-console/internal/syncclient"
	"github.com/spf13/cobra"
)

type configLoader func() (runtimeconfig.Runtime, error)
type serviceOpener func() (*audit.Service, error)

func New(stdout, stderr io.Writer) *cobra.Command {
	var dataDir, configFile string
	root := &cobra.Command{Use: "audit", Short: "Local-first AI agent audit console", SilenceUsage: true, SilenceErrors: true}
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.PersistentFlags().StringVar(&dataDir, "data-dir", "", "audit data directory (overrides configuration)")
	root.PersistentFlags().StringVar(&configFile, "config", "", "YAML config file (or AGENT_AUDIT_CONFIG)")
	loadConfig := func() (runtimeconfig.Runtime, error) {
		resolved, err := runtimeconfig.Load(configFile)
		if err != nil {
			return runtimeconfig.Runtime{}, err
		}
		if dataDir != "" {
			return resolved.WithDataDir(dataDir)
		}
		return resolved, nil
	}
	openService := func() (*audit.Service, error) {
		resolved, err := loadConfig()
		if err != nil {
			return nil, err
		}
		return audit.OpenWithConfig(resolved)
	}

	root.AddCommand(versionCommand(stdout), runCommand(stdout, stderr, openService), logCommand(stdout, openService), showCommand(stdout, openService), restoreCommand(stdout, openService), exportCommand(stdout, openService), accessTokenCommand(stdout, loadConfig), syncCommand(stdout, loadConfig), verifyCommand(stdout, openService), setupCommand(stdout, loadConfig), doctorCommand(stdout, loadConfig, openService), policyCommand(stdout, loadConfig, openService))
	return root
}

func setupCommand(stdout io.Writer, load configLoader) *cobra.Command {
	setup := &cobra.Command{Use: "setup", Short: "Preview or apply an agent MCP integration"}
	for _, value := range []integrations.Agent{integrations.Codex, integrations.ClaudeCode, integrations.Cursor} {
		agent := value
		var apply, dryRun, printOnly bool
		var configPath, commandPath string
		child := &cobra.Command{Use: string(agent), Short: "Configure Agent Audit MCP for " + string(agent), Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := load()
			if err != nil {
				return err
			}
			args := []string{"--data-dir", resolved.DataDir}
			plan, err := integrations.BuildPlan(agent, configPath, commandPath, args)
			if err != nil {
				return err
			}
			fmt.Fprint(stdout, integrations.Preview(plan))
			if !apply || dryRun || printOnly {
				fmt.Fprintln(stdout, "Preview only; use --apply to update the configuration.")
				return nil
			}
			result, err := integrations.Apply(plan)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "Applied atomically to %s\n", result.Plan.ConfigPath)
			if result.BackupPath != "" {
				fmt.Fprintf(stdout, "Backup: %s\n", result.BackupPath)
			}
			return nil
		}}
		child.Flags().BoolVar(&apply, "apply", false, "apply the configuration update")
		child.Flags().BoolVar(&dryRun, "dry-run", false, "preview without modifying files")
		child.Flags().BoolVar(&printOnly, "print", false, "print the proposed configuration change")
		child.Flags().StringVar(&configPath, "config-path", "", "override the agent configuration path")
		child.Flags().StringVar(&commandPath, "mcp-command", "", "absolute agent-audit-mcp executable path")
		setup.AddCommand(child)
	}
	return setup
}

func versionCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{Use: "version", Short: "Print release and toolchain version information", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		info := buildinfo.Current()
		fmt.Fprintf(stdout, "%s\nVersion: %s\nCommit: %s\nBuild Time: %s\nGo Version: %s\n", info.Product, info.Version, info.Commit, info.BuildTime, info.GoVersion)
		return nil
	}}
}

func runCommand(stdout, stderr io.Writer, open serviceOpener) *cobra.Command {
	var workdir, agentType, agentID string
	var approveHighRisk bool
	command := &cobra.Command{Use: "run -- COMMAND [ARG...]", Short: "Run and audit a command", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		approve := func(request audit.ApprovalRequest) bool {
			if approveHighRisk {
				return true
			}
			fmt.Fprintf(stderr, "High-risk action (%s, rule %s): %s\nApprove? [y/N]: ", request.Risk.Level, request.Decision.RuleID, strings.Join(service.Redactor.Strings(request.Command), " "))
			var answer string
			if _, err := fmt.Fscan(cmd.InOrStdin(), &answer); err != nil {
				return false
			}
			answer = strings.ToLower(strings.TrimSpace(answer))
			return answer == "y" || answer == "yes"
		}
		result, err := service.RunCommand(cmd.Context(), audit.RunOptions{Command: args, Workdir: workdir, AgentType: agentType, AgentID: agentID, Stdout: stdout, Stderr: stderr, Approve: approve})
		if err != nil {
			return err
		}
		fmt.Fprintln(stdout, service.StringResult(result))
		if result.ExitCode != 0 {
			code := result.ExitCode
			if code < 1 || code > 255 {
				code = 1
			}
			return CommandExitError{Code: code}
		}
		return nil
	}}
	command.Flags().StringVarP(&workdir, "workdir", "C", "", "working directory")
	command.Flags().StringVar(&agentType, "agent", "codex", "agent adapter: codex, claude-code, cursor, or custom")
	command.Flags().StringVar(&agentID, "agent-id", "audit-cli", "agent instance identifier")
	command.Flags().BoolVar(&approveHighRisk, "approve-high-risk", false, "approve matched high-risk policy rules non-interactively")
	return command
}

func logCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	return &cobra.Command{Use: "log", Short: "List audit runs", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		runs, err := service.Store.ListRuns(cmd.Context(), 100)
		if err != nil {
			return err
		}
		for _, run := range runs {
			fmt.Fprintf(stdout, "%s\t%s\t%s\t%s\n", run.RunID, run.Status, run.StartedAt.Format("2006-01-02 15:04:05"), run.WorkspacePath)
		}
		return nil
	}}
}

func showCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	return &cobra.Command{Use: "show RUN_ID", Short: "Show a run and its event timeline", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		summary, err := service.Summary(cmd.Context(), args[0])
		if err != nil {
			return readableRunError(args[0], err)
		}
		bundle, err := service.BuildBundle(cmd.Context(), args[0])
		if err != nil {
			return err
		}
		return json.NewEncoder(stdout).Encode(map[string]any{"summary": summary, "events": bundle.Events})
	}}
}

func restoreCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	var force bool
	command := &cobra.Command{Use: "restore FILE", Short: "Restore one file to its latest recorded pre-change state", Long: "Restore exactly one recorded file. The target must remain inside its enrolled workspace. A conflicting user edit is rejected unless --force is explicitly supplied.", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		path, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}
		record, err := service.RestoreWithOptions(cmd.Context(), path, audit.RestoreOptions{Force: force})
		if err != nil {
			if errors.Is(err, events.ErrNotFound) {
				return fmt.Errorf("no recorded snapshot was found for %s", path)
			}
			return err
		}
		fmt.Fprintf(stdout, "restored %s via %s\n", path, record.RollbackID)
		return nil
	}}
	command.Flags().BoolVar(&force, "force", false, "overwrite a current-state conflict explicitly")
	return command
}

func exportCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	var format, output string
	command := &cobra.Command{Use: "export RUN_ID", Short: "Export a self-contained JSON or HTML audit bundle", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		format = strings.ToLower(format)
		if format != "json" && format != "html" {
			return fmt.Errorf("unsupported export format %q (use json or html)", format)
		}
		if output == "" {
			output = args[0] + "." + format
		}
		if err := atomicOutput(output, func(file *os.File) error { return service.ExportRun(cmd.Context(), args[0], format, file) }); err != nil {
			return readableRunError(args[0], err)
		}
		fmt.Fprintf(stdout, "exported %s\n", output)
		return nil
	}}
	command.Flags().StringVar(&format, "format", "json", "export format: json or html")
	command.Flags().StringVarP(&output, "output", "o", "", "destination file (written atomically)")
	return command
}

func accessTokenCommand(stdout io.Writer, load configLoader) *cobra.Command {
	var name, role, path string
	command := &cobra.Command{Use: "access-token", Short: "Create an API token and store only its SHA-256 digest", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		resolved, err := load()
		if err != nil {
			return err
		}
		if path == "" {
			path = resolved.AccessFile
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		plain, err := auth.AddToken(path, name, auth.Role(role))
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "token=%s\naccess_file=%s\n", plain, path)
		return nil
	}}
	command.Flags().StringVar(&name, "name", "local-user", "token owner name")
	command.Flags().StringVar(&role, "role", string(auth.RoleViewer), "role: viewer, operator, or admin")
	command.Flags().StringVar(&path, "file", "", "access YAML path")
	return command
}

func syncCommand(stdout io.Writer, load configLoader) *cobra.Command {
	var endpoint, tokenEnv string
	var allowInsecure, legacyAllowHTTP bool
	command := &cobra.Command{Use: "sync RUN_ID", Short: "Explicitly send a verified audit bundle to a remote endpoint", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		resolved, err := load()
		if err != nil {
			return err
		}
		if endpoint == "" {
			endpoint = resolved.SyncEndpoint
		}
		if endpoint == "" {
			return errors.New("--endpoint or AGENT_AUDIT_SYNC_ENDPOINT is required")
		}
		if tokenEnv == "" {
			tokenEnv = resolved.SyncTokenEnv
		}
		token := os.Getenv(tokenEnv)
		if token == "" {
			return fmt.Errorf("sync token environment variable %s is empty", tokenEnv)
		}
		service, err := audit.OpenWithConfig(resolved)
		if err != nil {
			return err
		}
		defer service.Close()
		bundle, err := service.BuildBundle(cmd.Context(), args[0])
		if err != nil {
			return readableRunError(args[0], err)
		}
		receipt, err := (&syncclient.Client{Endpoint: endpoint, Token: token, AllowHTTP: allowInsecure || legacyAllowHTTP}).Push(cmd.Context(), bundle)
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "remote_status=%s remote_id=%s\n", receipt.Status, receipt.RemoteID)
		return nil
	}}
	command.Flags().StringVar(&endpoint, "endpoint", "", "remote base URL (HTTPS required)")
	command.Flags().StringVar(&tokenEnv, "token-env", "", "environment variable containing the bearer token")
	command.Flags().BoolVar(&allowInsecure, "allow-insecure", false, "allow plaintext HTTP for local development only")
	command.Flags().BoolVar(&legacyAllowHTTP, "allow-http", false, "deprecated alias for --allow-insecure")
	_ = command.Flags().MarkHidden("allow-http")
	return command
}

func verifyCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	return &cobra.Command{Use: "verify RUN_ID", Short: "Verify a run's complete event hash chain", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		if err := service.Store.VerifyRun(cmd.Context(), args[0]); err != nil {
			if errors.Is(err, events.ErrNotFound) {
				return readableRunError(args[0], err)
			}
			fmt.Fprintf(stdout, "BROKEN: %v\n", err)
			return CommandExitError{Code: 2, Message: "event hash chain is broken"}
		}
		fmt.Fprintln(stdout, "VALID")
		return nil
	}}
}

func policyCommand(stdout io.Writer, load configLoader, open serviceOpener) *cobra.Command {
	root := &cobra.Command{Use: "policy", Short: "Validate, simulate, explain, and test policy"}
	root.AddCommand(policyValidateCommand(stdout, load), policySimulateCommand(stdout, open), policyExplainCommand(stdout, open), policyTestCommand(stdout, load, open))
	return root
}

func policyValidateCommand(stdout io.Writer, load configLoader) *cobra.Command {
	var path string
	var jsonOutput bool
	command := &cobra.Command{Use: "validate", Short: "Validate a policy YAML document", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
		resolved, err := load()
		if err != nil {
			return err
		}
		if path == "" {
			path = resolved.PolicyFile
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		report := policy.ValidateDocument(data)
		if jsonOutput {
			_ = json.NewEncoder(stdout).Encode(report)
		} else {
			for _, item := range report.Errors {
				fmt.Fprintf(stdout, "ERROR [%s] %s\n", item.Code, item.Message)
			}
			for _, item := range report.Warnings {
				fmt.Fprintf(stdout, "WARNING [%s] %s\n", item.Code, item.Message)
			}
			if report.Valid {
				fmt.Fprintf(stdout, "PASS — %d rules; fingerprint %s\n", report.RuleCount, report.Fingerprint)
			}
		}
		if !report.Valid {
			return CommandExitError{Code: 2, Message: "policy validation failed"}
		}
		return nil
	}}
	command.Flags().StringVar(&path, "file", "", "policy YAML path")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON")
	return command
}

func policySimulateCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	var action, kind, path, agent, workspace, target, environment string
	var commandParts []string
	var jsonOutput bool
	command := &cobra.Command{Use: "simulate", Short: "Preview a policy decision without side effects", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		if action == "" {
			return errors.New("--action is required")
		}
		evidence, _ := json.Marshal(map[string]any{"command": commandParts, "path": path, "agent": agent, "workspace": workspace, "target": target, "environment": environment})
		result, err := service.Policy.Simulate(cmd.Context(), events.Event{Kind: events.Kind(kind), Intent: action, Evidence: events.Evidence{Data: evidence}, Reversibility: events.Reversibility{Status: events.NotApplicable}})
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(stdout).Encode(result)
		}
		fmt.Fprintf(stdout, "Action: %s\nRisk: %s\nDecision: %s\nEffective rule: %s\nReason: %s\nMatched rules:\n", action, strings.ToUpper(result.RiskLevel), strings.ToUpper(result.Decision), result.RuleID, result.Reason)
		for _, match := range result.MatchedRules {
			fmt.Fprintf(stdout, "- %s/%s\n", match.Source, match.RuleID)
		}
		return nil
	}}
	command.Flags().StringVar(&action, "action", "", "hypothetical action intent")
	command.Flags().StringVar(&kind, "kind", string(events.KindCommand), "event kind")
	command.Flags().StringSliceVar(&commandParts, "command", nil, "command and arguments")
	command.Flags().StringVar(&path, "path", "", "target path")
	command.Flags().StringVar(&agent, "agent", "", "agent identity")
	command.Flags().StringVar(&workspace, "workspace", "", "workspace context")
	command.Flags().StringVar(&target, "target", "", "target context")
	command.Flags().StringVar(&environment, "environment", "", "environment context")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON")
	return command
}

func policyExplainCommand(stdout io.Writer, open serviceOpener) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{Use: "explain RUN_ID ACTION_ID", Short: "Compare a recorded decision with current policy", Args: cobra.ExactArgs(2), RunE: func(cmd *cobra.Command, args []string) error {
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		items, err := service.Store.Query(cmd.Context(), events.Query{RunID: args[0], Limit: 1000})
		if err != nil {
			return err
		}
		var recorded *events.Event
		for index := range items {
			if items[index].ActionID == args[1] {
				item := items[index]
				recorded = &item
				break
			}
		}
		if recorded == nil {
			return fmt.Errorf("action %q was not found", args[1])
		}
		current, err := service.Policy.Simulate(cmd.Context(), *recorded)
		if err != nil {
			return err
		}
		value := map[string]any{"recorded": map[string]any{"decision": recorded.PolicyDecision, "risk": recorded.Risk, "policy_fingerprint": unavailable(recorded.PolicyDecision.PolicyFingerprint)}, "current": current, "re_evaluated": true, "policy_changed": recorded.PolicyDecision.PolicyFingerprint != "" && recorded.PolicyDecision.PolicyFingerprint != current.PolicyFingerprint}
		if jsonOutput {
			return json.NewEncoder(stdout).Encode(value)
		}
		fmt.Fprintf(stdout, "RECORDED DECISION: %s (%s)\nRECORDED POLICY: %s\nCURRENT SIMULATION: %s (%s)\nRE-EVALUATED: true\n", recorded.PolicyDecision.Status, recorded.Risk.Level, unavailable(recorded.PolicyDecision.PolicyFingerprint), current.Decision, current.RiskLevel)
		return nil
	}}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON")
	return command
}

func unavailable(value string) string {
	if value == "" {
		return "Unavailable"
	}
	return value
}

func policyTestCommand(stdout io.Writer, load configLoader, open serviceOpener) *cobra.Command {
	var path string
	var jsonOutput bool
	command := &cobra.Command{Use: "test", Short: "Run policy regression cases", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		resolved, err := load()
		if err != nil {
			return err
		}
		if path == "" {
			path = filepath.Join(filepath.Dir(resolved.PolicyFile), "policy.tests.yaml")
		}
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			data, err = policy.DefaultTests, nil
		}
		if err != nil {
			return err
		}
		service, err := open()
		if err != nil {
			return err
		}
		defer service.Close()
		report, err := policy.RunTests(cmd.Context(), service.Policy, data)
		if err != nil {
			return err
		}
		if jsonOutput {
			_ = json.NewEncoder(stdout).Encode(report)
		} else {
			for _, result := range report.Results {
				status := "PASS"
				if !result.Passed {
					status = "FAIL"
				}
				fmt.Fprintf(stdout, "%s %s — expected %s/%s, actual %s/%s\n", status, result.Name, result.Expected.Risk, result.Expected.Decision, result.Actual.Risk, result.Actual.Decision)
			}
			fmt.Fprintf(stdout, "%d passed, %d failed\n", report.Passed, report.Failed)
		}
		if report.Failed > 0 {
			return CommandExitError{Code: 3, Message: "policy tests failed"}
		}
		return nil
	}}
	command.Flags().StringVar(&path, "tests", "", "policy tests YAML path")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON")
	return command
}

func doctorCommand(stdout io.Writer, load configLoader, open serviceOpener) *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check local configuration and runtime dependencies without printing secrets", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		resolved, err := load()
		if err != nil {
			fmt.Fprintf(stdout, "FAIL config: %v\n", err)
			return err
		}
		fmt.Fprintln(stdout, "PASS config: resolved")
		service, err := open()
		if err != nil {
			fmt.Fprintf(stdout, "FAIL storage: %v\n", err)
			return err
		}
		defer service.Close()
		if err := service.Ready(cmd.Context()); err != nil {
			fmt.Fprintf(stdout, "FAIL readiness: %v\n", err)
			return err
		}
		version, err := service.Store.SchemaVersion(cmd.Context())
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "PASS data directory: writable\nPASS database: ready (schema %d)\nPASS snapshot directory: writable\n%s\n", version, service.Policy.Health())
		if resolved.RetentionDays == 0 {
			fmt.Fprintln(stdout, "PASS retention: keep forever")
		} else {
			fmt.Fprintf(stdout, "WARN retention: %d days configured; automatic deletion is not enabled in v0.2.0\n", resolved.RetentionDays)
		}
		if _, err := os.Stat(resolved.AccessFile); err == nil {
			if _, loadErr := auth.Load(resolved.AccessFile); loadErr != nil {
				fmt.Fprintf(stdout, "FAIL API access policy: %v\n", loadErr)
				return loadErr
			}
			fmt.Fprintln(stdout, "PASS API access policy: configured")
		} else if os.IsNotExist(err) {
			fmt.Fprintln(stdout, "WARN API access policy: not configured; keep auditd on loopback")
		} else {
			return err
		}
		if resolved.SyncEndpoint == "" {
			fmt.Fprintln(stdout, "WARN sync: endpoint not configured (sync remains disabled)")
		} else {
			parsed, parseErr := url.Parse(resolved.SyncEndpoint)
			if parseErr != nil || parsed.Host == "" {
				fmt.Fprintln(stdout, "FAIL sync: endpoint is not an absolute URL")
				return errors.New("invalid sync endpoint")
			}
			if parsed.Scheme == "https" {
				fmt.Fprintln(stdout, "PASS sync: HTTPS endpoint configured")
			} else {
				fmt.Fprintln(stdout, "FAIL sync: endpoint must use HTTPS")
				return errors.New("sync endpoint must use HTTPS")
			}
		}
		client := http.Client{Timeout: 500 * time.Millisecond}
		response, requestErr := client.Get("http://" + doctorAddress(resolved.ListenAddress) + "/healthz")
		if requestErr == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				fmt.Fprintln(stdout, "PASS auditd: health endpoint reachable")
			} else {
				fmt.Fprintf(stdout, "WARN auditd: health endpoint returned %s\n", response.Status)
			}
		} else {
			fmt.Fprintln(stdout, "WARN auditd: not reachable (start auditd when Web/API access is needed)")
		}
		if _, err := exec.LookPath("agent-audit-mcp"); err == nil {
			fmt.Fprintln(stdout, "PASS MCP: agent-audit-mcp found on PATH")
		} else {
			fmt.Fprintln(stdout, "WARN MCP: agent-audit-mcp not found on PATH")
		}
		fmt.Fprintln(stdout, "Agent Integration:")
		for _, agent := range []integrations.Agent{integrations.Codex, integrations.ClaudeCode, integrations.Cursor} {
			status, message := integrations.Check(agent, "")
			fmt.Fprintf(stdout, "%s %s: %s\n", status, agent, message)
		}
		return nil
	}}
}

func doctorAddress(address string) string {
	if strings.HasPrefix(address, ":") {
		return "127.0.0.1" + address
	}
	if strings.HasPrefix(address, "0.0.0.0:") {
		return "127.0.0.1:" + strings.TrimPrefix(address, "0.0.0.0:")
	}
	return address
}

func readableRunError(runID string, err error) error {
	if errors.Is(err, events.ErrNotFound) {
		return fmt.Errorf("run %q was not found", runID)
	}
	return err
}

func atomicOutput(path string, write func(*os.File) error) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(absolute), ".audit-export-*")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := write(temporary); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if _, err := os.Stat(absolute); os.IsNotExist(err) {
		return os.Rename(temporaryName, absolute)
	} else if err != nil {
		return err
	}
	backupFile, err := os.CreateTemp(filepath.Dir(absolute), ".audit-export-backup-*")
	if err != nil {
		return err
	}
	backup := backupFile.Name()
	if err := backupFile.Close(); err != nil {
		return err
	}
	if err := os.Remove(backup); err != nil {
		return err
	}
	if err := os.Rename(absolute, backup); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, absolute); err != nil {
		_ = os.Rename(backup, absolute)
		return err
	}
	return os.Remove(backup)
}

func Execute(ctx context.Context) int {
	command := New(os.Stdout, os.Stderr)
	command.SetContext(ctx)
	if err := command.Execute(); err != nil {
		var exitError CommandExitError
		if errors.As(err, &exitError) {
			if exitError.Message != "" {
				fmt.Fprintln(os.Stderr, "audit:", exitError.Message)
			}
			return exitError.Code
		}
		fmt.Fprintln(os.Stderr, "audit:", err)
		return 1
	}
	return 0
}

type CommandExitError struct {
	Code    int
	Message string
}

func (e CommandExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("command exited with code %d", e.Code)
}

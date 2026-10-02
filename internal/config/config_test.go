package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPriorityEnvironmentOverConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("data_dir: from-file\nlisten_address: 127.0.0.1:9000\nretention_days: 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AGENT_AUDIT_HOME", filepath.Join(t.TempDir(), "from-env"))
	value, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(value.DataDir) != "from-env" || value.ListenAddress != "127.0.0.1:9000" || value.RetentionDays != 7 {
		t.Fatalf("config=%+v", value)
	}
	overridden, err := value.WithDataDir(filepath.Join(t.TempDir(), "from-flag"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(overridden.DataDir) != "from-flag" {
		t.Fatalf("override=%+v", overridden)
	}
}

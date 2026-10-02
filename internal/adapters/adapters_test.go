package adapters

import (
	"github.com/agent-audit-console/agent-audit-console/internal/events"
	"testing"
)

func TestBuiltinAdaptersNormalizeToCoreEvent(t *testing.T) {
	registry := Builtins()
	if len(registry.List()) != 4 {
		t.Fatalf("adapters=%d", len(registry.List()))
	}
	for _, name := range []string{"codex", "claude-code", "cursor", "custom"} {
		value, err := registry.Resolve(name)
		if err != nil {
			t.Fatal(err)
		}
		event, err := value.Normalize(Action{RunID: "run-1", Kind: events.KindMCPCall, Intent: "record tool"})
		if err != nil {
			t.Fatal(err)
		}
		if event.Actor.Metadata["adapter"] != name {
			t.Fatalf("adapter=%s", event.Actor.Metadata["adapter"])
		}
	}
}

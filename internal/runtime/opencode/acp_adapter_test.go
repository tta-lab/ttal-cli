package opencode

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestACPAdapter_Runtime(t *testing.T) {
	cfg := runtime.AdapterConfig{
		AgentName: "test-agent",
		WorkDir:   "/tmp",
	}
	adapter := NewACPAdapter(cfg)
	if adapter.Runtime() != runtime.OpenCode {
		t.Errorf("expected OpenCode, got %s", adapter.Runtime())
	}
}

func TestACPAdapter_Config(t *testing.T) {
	cfg := runtime.AdapterConfig{
		AgentName: "my-agent",
		WorkDir:   "/Users/test/project",
		Port:      8080,
	}
	adapter := NewACPAdapter(cfg)
	if adapter == nil {
		t.Fatal("expected non-nil adapter")
	}
	if adapter.cfg.AgentName != "my-agent" {
		t.Errorf("expected agent name 'my-agent', got %s", adapter.cfg.AgentName)
	}
	if adapter.cfg.WorkDir != "/Users/test/project" {
		t.Errorf("expected workdir '/Users/test/project', got %s", adapter.cfg.WorkDir)
	}
}

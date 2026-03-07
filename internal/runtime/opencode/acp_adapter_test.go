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
	_ = adapter
}

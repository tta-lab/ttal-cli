package worker

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestValidateRuntime(t *testing.T) {
	// claude-code should fail (binary not in PATH during tests), but mention "claude"
	t.Run("claude-code valid", func(t *testing.T) {
		err := validateRuntime(runtime.ClaudeCode)
		if err != nil && !strings.Contains(err.Error(), "claude") {
			t.Errorf("error should mention claude binary, got: %v", err)
		}
	})
	// codex should be rejected as not a valid worker runtime
	t.Run("codex rejected", func(t *testing.T) {
		err := validateRuntime(runtime.Codex)
		if err == nil {
			t.Fatal("expected error for codex runtime")
		}
		if !strings.Contains(err.Error(), "cannot be used for workers") {
			t.Errorf("expected worker-rejection error, got: %v", err)
		}
	})
	// lenos should fail (binary not in PATH during tests), but mention "lenos"
	t.Run("lenos valid", func(t *testing.T) {
		err := validateRuntime(runtime.Lenos)
		if err != nil && !strings.Contains(err.Error(), "lenos") {
			t.Errorf("error should mention lenos binary, got: %v", err)
		}
	})
}

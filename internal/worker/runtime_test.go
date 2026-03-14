package worker

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestValidateRuntime(t *testing.T) {
	tests := []struct {
		name    string
		runtime runtime.Runtime
		wantBin string
	}{
		{"claude-code maps to claude binary", runtime.ClaudeCode, "claude"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRuntime(tt.runtime)
			// We only check that it returns a meaningful error mentioning the binary
			// since we can't control what's in PATH during tests
			if err != nil && tt.wantBin != "" {
				if !strings.Contains(err.Error(), tt.wantBin) {
					t.Errorf("validateRuntime(%q) error = %q, should mention %q", tt.runtime, err.Error(), tt.wantBin)
				}
			}
		})
	}
}

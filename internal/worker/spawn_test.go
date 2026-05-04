package worker

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestResolveRuntime(t *testing.T) {
	tests := []struct {
		name     string
		configRT runtime.Runtime
		want     runtime.Runtime
	}{
		{
			name:     "explicit claude-code",
			configRT: runtime.ClaudeCode,
			want:     runtime.ClaudeCode,
		},
		{
			name:     "explicit lenos",
			configRT: runtime.Lenos,
			want:     runtime.Lenos,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveRuntime(tt.configRT, nil)
			if got != tt.want {
				t.Errorf("resolveRuntime(%q) = %q, want %q", tt.configRT, got, tt.want)
			}
		})
	}
}

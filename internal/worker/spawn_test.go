package worker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
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

func TestResolveWorkerPairWith_CoderUsesTaskOwnerUDA(t *testing.T) {
	got := resolveWorkerPairWith(&config.Config{}, "coder", "astra")
	if got != "astra" {
		t.Errorf("expected coder to pair with task owner astra, got %q", got)
	}
}

func TestResolveWorkerPairWith_CoderWithoutOwnerReturnsEmpty(t *testing.T) {
	got := resolveWorkerPairWith(&config.Config{}, "coder", "")
	if got != "" {
		t.Errorf("expected empty pair target without task owner, got %q", got)
	}
}

func TestResolveWorkerPairWith_NonCoderUsesFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "worker-reviewer")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\nname: worker-reviewer\nrole: worker\nlenos:\n  pair_with: coder\n---\n# Worker Reviewer\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Sync: config.SyncConfig{WorkerAgentPaths: []string{tmpDir}}}

	got := resolveWorkerPairWith(cfg, "worker-reviewer", "astra")
	if got != "coder" {
		t.Errorf("expected non-coder to use frontmatter pair target coder, got %q", got)
	}
}

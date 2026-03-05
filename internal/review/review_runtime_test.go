package review

import (
	"fmt"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerRuntimeCmd(t *testing.T) {
	tests := []struct {
		name string
		rt   runtime.Runtime
		want string
	}{
		{
			name: "claude-code uses claude with model and yolo",
			rt:   runtime.ClaudeCode,
			want: "ttal worker gatekeeper --task-file /tmp/prompt.txt -- claude --model opus --dangerously-skip-permissions --",
		},
		{
			name: "opencode uses opencode prompt mode",
			rt:   runtime.OpenCode,
			want: "ttal worker gatekeeper --task-file /tmp/prompt.txt -- opencode --prompt",
		},
		{
			name: "codex uses codex yolo prompt mode",
			rt:   runtime.Codex,
			want: "ttal worker gatekeeper --task-file /tmp/prompt.txt -- codex --yolo --prompt",
		},
		{
			name: "non-worker runtime falls back to claude",
			rt:   runtime.OpenClaw,
			want: "ttal worker gatekeeper --task-file /tmp/prompt.txt -- claude --model opus --dangerously-skip-permissions --",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildReviewerRuntimeCmd("ttal", "/tmp/prompt.txt", tt.rt)
			if got != tt.want {
				t.Fatalf("unexpected command\nwant: %s\n got: %s", tt.want, got)
			}
		})
	}
}

func TestBuildReviewerRuntimeCmd_InterpolatesPaths(t *testing.T) {
	ttalBin := "/usr/local/bin/ttal"
	promptFile := "/tmp/review-123.txt"
	got := buildReviewerRuntimeCmd(ttalBin, promptFile, runtime.ClaudeCode)
	wantPrefix := fmt.Sprintf("%s worker gatekeeper --task-file %s", ttalBin, promptFile)
	if got[:len(wantPrefix)] != wantPrefix {
		t.Fatalf("command should start with %q, got %q", wantPrefix, got)
	}
}

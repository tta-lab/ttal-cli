package review

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerRuntimeCmd(t *testing.T) {
	tests := []struct {
		name  string
		rt    runtime.Runtime
		model string
		want  string
		err   bool
	}{
		{
			name:  "claude-code uses sonnet by default",
			rt:    runtime.ClaudeCode,
			model: "sonnet",
			want:  "ttal worker gatekeeper --task-file /tmp/prompt.txt -- claude --model sonnet --dangerously-skip-permissions --",
		},
		{
			name:  "claude-code with opus model",
			rt:    runtime.ClaudeCode,
			model: "opus",
			want:  "ttal worker gatekeeper --task-file /tmp/prompt.txt -- claude --model opus --dangerously-skip-permissions --",
		},
		{
			name:  "opencode ignores model",
			rt:    runtime.OpenCode,
			model: "sonnet",
			want:  "ttal worker gatekeeper --task-file /tmp/prompt.txt -- opencode --prompt",
		},
		{
			name:  "codex ignores model",
			rt:    runtime.Codex,
			model: "sonnet",
			want:  "ttal worker gatekeeper --task-file /tmp/prompt.txt -- codex --yolo --prompt",
		},
		{
			name: "non-worker runtime errors",
			rt:   runtime.OpenClaw,
			err:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := buildReviewerRuntimeCmd("ttal", "/tmp/prompt.txt", tt.rt, tt.model)
			if tt.err {
				if err == nil {
					t.Fatalf("expected error, got command: %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("unexpected command\nwant: %s\n got: %s", tt.want, got)
			}
		})
	}
}

func TestBuildReviewerRuntimeCmd_InterpolatesPaths(t *testing.T) {
	ttalBin := "/usr/local/bin/ttal"
	promptFile := "/tmp/review-123.txt"
	got, err := buildReviewerRuntimeCmd(ttalBin, promptFile, runtime.ClaudeCode, "sonnet")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantPrefix := fmt.Sprintf("%s worker gatekeeper --task-file %s", ttalBin, promptFile)
	if !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("command should start with %q, got %q", wantPrefix, got)
	}
}

package config

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestRenderSkillPlaceholders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		rt    runtime.Runtime
		want  string
	}{
		{
			name:  "CC replaces skill placeholder with text",
			input: "Write a plan for task {{task-id}}",
			rt:    runtime.ClaudeCode,
			want:  "Write a plan for task abc123",
		},
		{
			name:  "CC replaces skill placeholder at start",
			input: "{{skill:sp-writing-plans}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.ClaudeCode,
			want:  "Use sp-writing-plans skill\n\nWrite a plan for task abc123",
		},
		{
			name:  "Codex replaces skill placeholder with dollar",
			input: "{{skill:sp-writing-plans}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.Codex,
			want:  "$sp-writing-plans\n\nWrite a plan for task abc123",
		},
		{
			name:  "OC replaces skill placeholder with text",
			input: "{{skill:pr-review}}\nReview this PR",
			rt:    runtime.OpenCode,
			want:  "Use pr-review skill\n\nReview this PR",
		},
		{
			name:  "multiple skill placeholders",
			input: "{{skill:sp-writing-plans}}\n{{skill:flicknote-cli}}\nDo the thing",
			rt:    runtime.Codex,
			want:  "$sp-writing-plans\n$flicknote-cli\n\nDo the thing",
		},
		{
			name:  "no placeholders unchanged",
			input: "Just a plain prompt",
			rt:    runtime.ClaudeCode,
			want:  "Just a plain prompt",
		},
		{
			name:  "empty skill name removed",
			input: "{{skill:}}\nSome text",
			rt:    runtime.ClaudeCode,
			want:  "Some text",
		},
		{
			name:  "unclosed skill placeholder left unchanged",
			input: "{{skill:broken\nSome text",
			rt:    runtime.ClaudeCode,
			want:  "{{skill:broken\nSome text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderTemplate(tt.input, "abc123", tt.rt)
			if got != tt.want {
				t.Errorf("RenderTemplate() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestPromptFallbackDefaults(t *testing.T) {
	cfg := &Config{} // empty prompts — should fall back to defaults (or roles.toml if exists)

	// Test that prompts are returned for known keys
	// Note: Prompt() returns raw templates - rendering happens in RenderPrompt()
	keys := []string{"designer", "researcher", "execute", "triage", "review", "re_review"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			got := cfg.Prompt(key)
			if got == "" {
				t.Errorf("Prompt(%q) = empty string, want non-empty", key)
			}
		})
	}

	// Unknown key should return empty
	t.Run("unknown", func(t *testing.T) {
		got := cfg.Prompt("unknown")
		if got != "" {
			t.Errorf("Prompt(%q) = %q, want empty string", "unknown", got)
		}
	})
}

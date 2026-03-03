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
			name:  "CC replaces skill placeholder with slash",
			input: "{{skill:sp-writing-plans}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.ClaudeCode,
			want:  "/sp-writing-plans\nWrite a plan for task abc123",
		},
		{
			name:  "Codex replaces skill placeholder with dollar",
			input: "{{skill:sp-writing-plans}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.Codex,
			want:  "$sp-writing-plans\nWrite a plan for task abc123",
		},
		{
			name:  "OC replaces skill placeholder with slash",
			input: "{{skill:pr-review}}\nReview this PR",
			rt:    runtime.OpenCode,
			want:  "/pr-review\nReview this PR",
		},
		{
			name:  "multiple skill placeholders",
			input: "{{skill:sp-writing-plans}}\n{{skill:flicknote-cli}}\nDo the thing",
			rt:    runtime.Codex,
			want:  "$sp-writing-plans\n$flicknote-cli\nDo the thing",
		},
		{
			name:  "no placeholders unchanged",
			input: "Just a plain prompt",
			rt:    runtime.ClaudeCode,
			want:  "Just a plain prompt",
		},
		{
			name:  "empty skill name left unchanged",
			input: "{{skill:}}\nSome text",
			rt:    runtime.ClaudeCode,
			want:  "{{skill:}}\nSome text",
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
	cfg := &Config{} // empty prompts — should fall back to defaults
	defaults := DefaultPrompts()

	tests := []struct {
		key  string
		want string
	}{
		{"design", defaults.Design},
		{"research", defaults.Research},
		{"test", defaults.Test},
		{"execute", defaults.Execute},
		{"triage", defaults.Triage},
		{"review", defaults.Review},
		{"re_review", defaults.ReReview},
		{"unknown", ""},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			got := cfg.Prompt(tt.key)
			if got != tt.want {
				t.Errorf("Prompt(%q) length = %d, want %d", tt.key, len(got), len(tt.want))
			}
		})
	}
}

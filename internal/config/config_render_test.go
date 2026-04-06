package config

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/skill"
)

func TestRenderSkillPlaceholders(t *testing.T) {
	// Mock flicknote fetcher so tests don't depend on real flicknote content.
	orig := skill.FlicknoteFetcher
	t.Cleanup(func() { skill.FlicknoteFetcher = orig })
	skill.FlicknoteFetcher = func(id string) (string, error) {
		return "SKILL_CONTENT(" + id + ")", nil
	}

	tests := []struct {
		name  string
		input string
		rt    runtime.Runtime
		want  string
	}{
		{
			name:  "CC replaces task-id placeholder",
			input: "Write a plan for task {{task-id}}",
			rt:    runtime.ClaudeCode,
			want:  "Write a plan for task abc123",
		},
		{
			name:  "skill placeholder replaced with content",
			input: "{{skill:sp-planning}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.ClaudeCode,
			want:  "# sp-planning [skill]\n\nSKILL_CONTENT(cd32f690)\n\nWrite a plan for task abc123",
		},
		{
			name:  "skill placeholder replaced regardless of runtime",
			input: "{{skill:sp-planning}}\nWrite a plan for task {{task-id}}",
			rt:    runtime.Codex,
			want:  "# sp-planning [skill]\n\nSKILL_CONTENT(cd32f690)\n\nWrite a plan for task abc123",
		},
		{
			name:  "multiple skill placeholders",
			input: "{{skill:sp-planning}}\n{{skill:flicknote}}\nDo the thing",
			rt:    runtime.Codex,
			want: "# sp-planning [skill]\n\nSKILL_CONTENT(cd32f690)" +
				"\n\n# flicknote [skill]\n\nSKILL_CONTENT(8977bddf)\n\nDo the thing",
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
		{
			name:  "skill placeholder in middle of text",
			input: "Start {{skill:triage}} middle end",
			rt:    runtime.ClaudeCode,
			want:  "# triage [skill]\n\nSKILL_CONTENT(c64be429)\n\nStart  middle end",
		},
		{
			name:  "skill placeholder at end of text",
			input: "Some text {{skill:triage}}",
			rt:    runtime.Codex,
			want:  "# triage [skill]\n\nSKILL_CONTENT(c64be429)\n\nSome text ",
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

func TestPromptWorkerKeysSkipRolesDefault(t *testing.T) {
	cfg := &Config{
		resolvedRoles: &RolesConfig{
			Roles: map[string]string{
				"default": "manager system prompt",
			},
		},
	}
	// Worker-plane keys must NOT inherit [default]
	for _, key := range []string{"context", "review", "re_review", "triage"} {
		if got := cfg.Prompt(key); got != "" {
			t.Errorf("Prompt(%q) = %q, want empty (must not inherit [default])", key, got)
		}
	}
	// Manager-plane key MUST inherit [default]
	if got := cfg.Prompt("designer"); got != "manager system prompt" {
		t.Errorf("Prompt(\"designer\") = %q, want \"manager system prompt\"", got)
	}
}

func TestPromptNoDefaults(t *testing.T) {
	// Use a temp HOME dir so LoadRoles finds no roles.toml, making the test hermetic
	// regardless of whether the developer has a real roles.toml installed.
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	cfg := &Config{} // empty prompts, no resolvedRoles — no fallback to defaults

	keys := []string{"designer", "researcher", "execute", "triage", "review", "re_review"}
	for _, key := range keys {
		t.Run(key, func(t *testing.T) {
			got := cfg.Prompt(key)
			if got != "" {
				t.Errorf("Prompt(%q) = %q, want empty string (no defaults without roles.toml)", key, got)
			}
		})
	}

	t.Run("unknown", func(t *testing.T) {
		got := cfg.Prompt("unknown")
		if got != "" {
			t.Errorf("Prompt(%q) = %q, want empty string", "unknown", got)
		}
	})
}

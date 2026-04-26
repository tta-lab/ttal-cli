package config

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestRenderTemplate_TaskID(t *testing.T) {
	got := RenderTemplate("Write a plan for task {{task-id}}", "abc123", runtime.ClaudeCode)
	want := "Write a plan for task abc123"
	if got != want {
		t.Errorf("RenderTemplate() = %q, want %q", got, want)
	}
}

func TestPromptWorkerKeysSkipRolesDefault(t *testing.T) {
	cfg := &Config{
		Roles: &RolesConfig{
			Roles: map[string]string{
				"default": "manager system prompt",
			},
		},
	}
	// Worker-plane keys must NOT inherit [default]
	for _, key := range []string{"context_manager", "context_worker", "review", "re_review", "triage"} {
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

package gitutil

import (
	"testing"
)

func TestTokenForRemote(t *testing.T) {
	tests := []struct {
		name      string
		remoteURL string
		envKey    string
		envVal    string
		wantToken string
	}{
		{"github https", "https://github.com/org/repo.git", "GITHUB_TOKEN", "gh-tok", "gh-tok"},
		{"forgejo https", "https://git.guion.io/org/repo.git", "FORGEJO_TOKEN", "fg-tok", "fg-tok"},
		{"github ssh-style", "git@github.com:org/repo.git", "GITHUB_TOKEN", "gh-tok2", "gh-tok2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			got := tokenForRemote(tt.remoteURL, "")
			if got != tt.wantToken {
				t.Errorf("tokenForRemote(%q) = %q, want %q", tt.remoteURL, got, tt.wantToken)
			}
		})
	}
}

func TestGitCredEnv(t *testing.T) {
	t.Run("with token includes all 7 env vars", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "test-token-123")
		env := GitCredEnv("https://github.com/org/repo", "")
		if len(env) != 7 {
			t.Fatalf("expected 7 env vars, got %d: %v", len(env), env)
		}
		if env[0] != "GIT_TERMINAL_PROMPT=0" {
			t.Errorf("env[0] = %q, want GIT_TERMINAL_PROMPT=0", env[0])
		}
		if env[1] != "GIT_CONFIG_COUNT=2" {
			t.Errorf("env[1] = %q, want GIT_CONFIG_COUNT=2", env[1])
		}
		// Token is passed via GIT_TOKEN_INJECT, not interpolated into the shell string.
		if env[6] != "GIT_TOKEN_INJECT=test-token-123" {
			t.Errorf("env[6] = %q, want GIT_TOKEN_INJECT=test-token-123", env[6])
		}
	})

	t.Run("without token still returns GIT_TERMINAL_PROMPT=0", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("FORGEJO_TOKEN", "")
		env := GitCredEnv("https://github.com/org/repo", "")
		if len(env) != 1 {
			t.Fatalf("expected 1 env var, got %d: %v", len(env), env)
		}
		if env[0] != "GIT_TERMINAL_PROMPT=0" {
			t.Errorf("env[0] = %q, want GIT_TERMINAL_PROMPT=0", env[0])
		}
	})

	t.Run("forgejo without token returns prompt suppression only", func(t *testing.T) {
		t.Setenv("FORGEJO_TOKEN", "")
		env := GitCredEnv("https://git.guion.io/org/repo", "")
		if len(env) != 1 {
			t.Fatalf("expected 1 env var, got %d", len(env))
		}
	})
}

func TestGitCredEnvHasToken(t *testing.T) {
	t.Run("returns true when token available", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "mytoken")
		if !GitCredEnvHasToken("https://github.com/org/repo", "") {
			t.Error("expected true, got false")
		}
	})

	t.Run("returns false when no token", func(t *testing.T) {
		t.Setenv("GITHUB_TOKEN", "")
		t.Setenv("FORGEJO_TOKEN", "")
		if GitCredEnvHasToken("https://github.com/org/repo", "") {
			t.Error("expected false, got true")
		}
	})
}

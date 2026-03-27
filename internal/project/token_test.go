package project

import (
	"path/filepath"
	"testing"
)

const testTokenEnvVar = "GUION_GITHUB_TOKEN"
const testTokenValue = "ghp_guion_test123"
const testGlobalToken = "ghp_global_test456"

// newTestStoreWithProject creates a temp store and adds a project with the given alias and github_token_env.
func newTestStoreWithProject(t *testing.T, alias, tokenEnv string) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "projects.toml")
	s := NewStore(path)
	if err := s.Add(alias, "Test Project", "/path/test"); err != nil {
		t.Fatalf("Add(%q) error: %v", alias, err)
	}
	if tokenEnv != "" {
		if err := s.Modify(alias, map[string]string{"github_token_env": tokenEnv}); err != nil {
			t.Fatalf("Modify(%q, github_token_env) error: %v", alias, err)
		}
	}
	return s
}

func TestResolveGitHubTokenWithOverride(t *testing.T) {
	t.Setenv(testTokenEnvVar, testTokenValue)
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	s := newTestStoreWithProject(t, "guion", testTokenEnvVar)
	token := resolveGitHubTokenWithStore("guion", s)
	if token != testTokenValue {
		t.Errorf("token = %q, want %q", token, testTokenValue)
	}
}

func TestResolveGitHubTokenEnvVarEmpty(t *testing.T) {
	t.Setenv(testTokenEnvVar, "") // override env var is empty
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	s := newTestStoreWithProject(t, "guion", testTokenEnvVar)
	token := resolveGitHubTokenWithStore("guion", s)
	// Should fall back to global GITHUB_TOKEN
	if token != testGlobalToken {
		t.Errorf("token = %q, want %q (global fallback)", token, testGlobalToken)
	}
}

func TestResolveGitHubTokenNoOverrideConfigured(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	// Project has no github_token_env set
	s := newTestStoreWithProject(t, "guion", "")
	token := resolveGitHubTokenWithStore("guion", s)
	if token != testGlobalToken {
		t.Errorf("token = %q, want %q (global fallback)", token, testGlobalToken)
	}
}

func TestResolveGitHubTokenEmptyAlias(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	s := newTestStoreWithProject(t, "guion", testTokenEnvVar)
	token := resolveGitHubTokenWithStore("", s)
	if token != testGlobalToken {
		t.Errorf("token = %q, want %q (global fallback)", token, testGlobalToken)
	}
}

func TestResolveGitHubTokenNonExistentAlias(t *testing.T) {
	t.Setenv(testTokenEnvVar, "") // clear any real env value so single-project shortcut doesn't use it
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	s := newTestStoreWithProject(t, "guion", testTokenEnvVar)
	token := resolveGitHubTokenWithStore("does-not-exist", s)
	if token != testGlobalToken {
		t.Errorf("token = %q, want %q (global fallback)", token, testGlobalToken)
	}
}

func TestResolveGitHubTokenHierarchicalResolution(t *testing.T) {
	// Critical: project alias "ttal", query with "ttal.pr" → should resolve to override
	t.Setenv(testTokenEnvVar, testTokenValue)
	t.Setenv("GITHUB_TOKEN", testGlobalToken)

	s := newTestStoreWithProject(t, "ttal", testTokenEnvVar)
	token := resolveGitHubTokenWithStore("ttal.pr", s)
	if token != testTokenValue {
		t.Errorf("hierarchical: token = %q, want %q", token, testTokenValue)
	}
}

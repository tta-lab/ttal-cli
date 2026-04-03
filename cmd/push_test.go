package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/project"
)

func TestResolveAliasFromPath_RegisteredProject(t *testing.T) {
	// Exact path match
	orig := storeFactoryFn
	t.Cleanup(func() { storeFactoryFn = orig })

	tmpDir := t.TempDir()
	store := project.NewStore(filepath.Join(tmpDir, "projects.toml"))
	if err := store.Add("testproj", "Test Project", filepath.Join(tmpDir, "code")); err != nil {
		t.Fatalf("failed to add project: %v", err)
	}
	storeFactoryFn = func() *project.Store { return store }

	// Exact match
	got := resolveAliasFromPath(filepath.Join(tmpDir, "code"))
	if got != "testproj" {
		t.Errorf("resolveAliasFromPath(exact) = %q, want %q", got, "testproj")
	}

	// Nested inside registered path
	subDir := filepath.Join(tmpDir, "code", "backend", "cmd")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got = resolveAliasFromPath(subDir)
	if got != "testproj" {
		t.Errorf("resolveAliasFromPath(nested) = %q, want %q", got, "testproj")
	}
}

func TestResolveAliasFromPath_UnregisteredPath(t *testing.T) {
	orig := storeFactoryFn
	t.Cleanup(func() { storeFactoryFn = orig })

	tmpDir := t.TempDir()
	store := project.NewStore(filepath.Join(tmpDir, "projects.toml"))
	if err := store.Add("other", "Other", filepath.Join(tmpDir, "other-code")); err != nil {
		t.Fatalf("failed to add project: %v", err)
	}
	storeFactoryFn = func() *project.Store { return store }

	// Unregistered path — no match
	got := resolveAliasFromPath(filepath.Join(tmpDir, "unregistered"))
	if got != "" {
		t.Errorf("resolveAliasFromPath(unregistered) = %q, want %q", got, "")
	}
}

func TestResolveAliasFromPath_WorktreePath(t *testing.T) {
	orig := storeFactoryFn
	t.Cleanup(func() { storeFactoryFn = orig })

	tmpDir := t.TempDir()
	worktreesRoot := filepath.Join(tmpDir, "worktrees")
	if err := os.MkdirAll(worktreesRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	store := project.NewStore(filepath.Join(tmpDir, "projects.toml"))
	if err := store.Add("myproj", "My Project", "/real/myproj/path"); err != nil {
		t.Fatalf("failed to add project: %v", err)
	}
	storeFactoryFn = func() *project.Store { return store }

	// Worktree path: ~/.ttal/worktrees/<uuid8>-<alias>/...
	// The worktree fallback only matches when the last path component (base)
	// contains a hyphen — e.g. "abc12345-myproj", not "src".
	worktreeRoot := filepath.Join(worktreesRoot, "abc12345-myproj")
	if err := os.MkdirAll(worktreeRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got := resolveAliasFromPath(worktreeRoot)
	if got != "myproj" {
		t.Errorf("resolveAliasFromPath(worktree root) = %q, want %q", got, "myproj")
	}

	// Nested inside worktree root — no hyphen in base, so worktree fallback fails
	subDir := filepath.Join(worktreeRoot, "src")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got = resolveAliasFromPath(subDir)
	if got != "" {
		t.Errorf("resolveAliasFromPath(nested worktree) = %q, want %q", got, "")
	}

	// Worktree with unknown alias — not registered, returns ""
	unknownRoot := filepath.Join(worktreesRoot, "abc12345-unknownproj")
	if err := os.MkdirAll(unknownRoot, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	got = resolveAliasFromPath(unknownRoot)
	if got != "" {
		t.Errorf("resolveAliasFromPath(unknown worktree) = %q, want %q", got, "")
	}
}

func TestResolveAliasFromPath_NilProjectsStore(t *testing.T) {
	orig := storeFactoryFn
	t.Cleanup(func() { storeFactoryFn = orig })

	// Store that returns error on List
	store := project.NewStore("/nonexistent/path/projects.toml")
	storeFactoryFn = func() *project.Store { return store }

	got := resolveAliasFromPath("/any/path")
	if got != "" {
		t.Errorf("resolveAliasFromPath(store error) = %q, want %q", got, "")
	}
}

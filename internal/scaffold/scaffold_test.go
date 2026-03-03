package scaffold

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")

	_ = os.MkdirAll(filepath.Join(src, "agent"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "config.toml"), []byte("test"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "agent", "CLAUDE.md"), []byte("# Agent"), 0o644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for _, rel := range []string{"config.toml", "agent/CLAUDE.md"} {
		if _, err := os.Stat(filepath.Join(dst, rel)); err != nil {
			t.Errorf("expected %s to exist", rel)
		}
	}
}

func TestCopyDirSkipsDotDirs(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "out")

	_ = os.MkdirAll(filepath.Join(src, ".git", "objects"), 0o755)
	_ = os.WriteFile(filepath.Join(src, ".git", "HEAD"), []byte("ref"), 0o644)
	_ = os.WriteFile(filepath.Join(src, "README.md"), []byte("hi"), 0o644)

	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".git")); !os.IsNotExist(err) {
		t.Error("expected .git to be skipped")
	}
	if _, err := os.Stat(filepath.Join(dst, "README.md")); err != nil {
		t.Error("expected README.md to exist")
	}
}

func TestApplyScaffold(t *testing.T) {
	repoDir := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "ws")

	// Create scaffold
	scaffoldDir := filepath.Join(repoDir, "basic")
	_ = os.MkdirAll(filepath.Join(scaffoldDir, "manager"), 0o755)
	_ = os.WriteFile(filepath.Join(scaffoldDir, "config.toml"), []byte(`default_team = "default"`), 0o644)
	_ = os.WriteFile(filepath.Join(scaffoldDir, "README.md"), []byte("# Basic — Two agents\n\nA minimal setup."), 0o644)
	_ = os.WriteFile(filepath.Join(scaffoldDir, "manager", "CLAUDE.md"), []byte("# Manager"), 0o644)

	// Create shared docs
	docsDir := filepath.Join(repoDir, "docs", "skills", "git-omz")
	_ = os.MkdirAll(docsDir, 0o755)
	_ = os.WriteFile(filepath.Join(docsDir, "SKILL.md"), []byte("# Git"), 0o644)

	err := Apply(repoDir, "basic", workspace)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if _, err := os.Stat(filepath.Join(workspace, "manager", "CLAUDE.md")); err != nil {
		t.Error("expected manager/CLAUDE.md")
	}
	if _, err := os.Stat(filepath.Join(workspace, "docs", "skills", "git-omz", "SKILL.md")); err != nil {
		t.Error("expected docs/skills/git-omz/SKILL.md")
	}
}

func TestApplyInvalidScaffold(t *testing.T) {
	repoDir := t.TempDir()
	workspace := filepath.Join(t.TempDir(), "ws")

	err := Apply(repoDir, "nonexistent", workspace)
	if err == nil {
		t.Error("expected error for nonexistent scaffold")
	}
}

func TestListScaffoldsFromHeadings(t *testing.T) {
	repoDir := t.TempDir()

	// basic scaffold with heading-style README
	basic := filepath.Join(repoDir, "basic")
	_ = os.MkdirAll(filepath.Join(basic, "manager"), 0o755)
	_ = os.WriteFile(filepath.Join(basic, "config.toml"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(basic, "README.md"), []byte("# Basic — Two agents\n\nA minimal setup."), 0o644)
	_ = os.WriteFile(filepath.Join(basic, "manager", "CLAUDE.md"), []byte(""), 0o644)

	// Non-scaffold dir (no config.toml)
	_ = os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755)

	scaffolds, err := List(repoDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scaffolds) != 1 {
		t.Fatalf("expected 1 scaffold, got %d", len(scaffolds))
	}

	if scaffolds[0].Name != "Basic" {
		t.Errorf("expected name 'Basic', got %q", scaffolds[0].Name)
	}
	if scaffolds[0].Dir != "basic" {
		t.Errorf("expected dir 'basic', got %q", scaffolds[0].Dir)
	}
	if scaffolds[0].Description != "Two agents" {
		t.Errorf("expected description 'Two agents', got %q", scaffolds[0].Description)
	}
	if scaffolds[0].Agents != "manager" {
		t.Errorf("expected agents 'manager', got %q", scaffolds[0].Agents)
	}
}

func TestListScaffoldsWithFrontmatter(t *testing.T) {
	repoDir := t.TempDir()

	ff := filepath.Join(repoDir, "full-flicknote")
	_ = os.MkdirAll(filepath.Join(ff, "hawk"), 0o755)
	_ = os.WriteFile(filepath.Join(ff, "config.toml"), []byte(""), 0o644)
	fmContent := "---\nname: Full (FlickNote)\n" +
		"description: With personalities\n" +
		"install_hint: Requires FlickNote CLI\n---\n# Full"
	_ = os.WriteFile(filepath.Join(ff, "README.md"), []byte(fmContent), 0o644)
	_ = os.WriteFile(filepath.Join(ff, "hawk", "CLAUDE.md"), []byte(""), 0o644)

	scaffolds, err := List(repoDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scaffolds) != 1 {
		t.Fatalf("expected 1 scaffold, got %d", len(scaffolds))
	}

	if scaffolds[0].Name != "Full (FlickNote)" {
		t.Errorf("expected name 'Full (FlickNote)', got %q", scaffolds[0].Name)
	}
	if scaffolds[0].InstallHint != "Requires FlickNote CLI" {
		t.Errorf("expected install_hint, got %q", scaffolds[0].InstallHint)
	}
}

func TestListMultipleScaffolds(t *testing.T) {
	repoDir := t.TempDir()

	for _, name := range []string{"basic", "full-markdown"} {
		dir := filepath.Join(repoDir, name)
		_ = os.MkdirAll(dir, 0o755)
		_ = os.WriteFile(filepath.Join(dir, "config.toml"), []byte(""), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "README.md"), []byte("# "+name), 0o644)
	}

	// Non-scaffold dirs
	_ = os.MkdirAll(filepath.Join(repoDir, "docs"), 0o755)
	_ = os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755)

	scaffolds, err := List(repoDir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(scaffolds) != 2 {
		t.Fatalf("expected 2 scaffolds, got %d", len(scaffolds))
	}
	// Sorted alphabetically
	if scaffolds[0].Dir != "basic" {
		t.Errorf("expected first scaffold 'basic', got %q", scaffolds[0].Dir)
	}
}

package agentfs

import (
	"os"
	"path/filepath"
	"testing"
)

// writeAgentWithAccess creates a {name}/AGENTS.md file in dir with the given access value.
// An empty access omits the field from frontmatter.
func writeAgentWithAccess(t *testing.T, dir, name, access string) {
	t.Helper()
	agentDir := filepath.Join(dir, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\nname: " + name + "\nrole: reviewer\n"
	if access != "" {
		fm += "lenos:\n  access: " + access + "\n"
	}
	fm += "---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveAccess_ReadsRO verifies that lenos.access: ro in frontmatter
// is returned verbatim.
func TestResolveAccess_ReadsRO(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithAccess(t, tmpDir, "pr-review-lead", "ro")

	got := ResolveAccess("", []string{tmpDir}, "pr-review-lead")
	if got != "ro" {
		t.Errorf("expected ro, got %q", got)
	}
}

// TestResolveAccess_ReadsRW verifies that lenos.access: rw in frontmatter
// is returned verbatim.
func TestResolveAccess_ReadsRW(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithAccess(t, tmpDir, "coder", "rw")

	got := ResolveAccess("", []string{tmpDir}, "coder")
	if got != "rw" {
		t.Errorf("expected rw, got %q", got)
	}
}

// TestResolveAccess_FallbackOnMissingAgent verifies that when no agent
// is found in search paths, the default "rw" is returned.
func TestResolveAccess_FallbackOnMissingAgent(t *testing.T) {
	tmpDir := t.TempDir() // empty — no agent subdirs

	got := ResolveAccess("", []string{tmpDir}, "ghost")
	if got != "rw" {
		t.Errorf("expected fallback rw, got %q", got)
	}
}

// TestResolveAccess_FallbackOnEmptyAccess verifies that when frontmatter
// has no lenos.access field, the default "rw" is returned.
func TestResolveAccess_FallbackOnEmptyAccess(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithAccess(t, tmpDir, "neutral", "")

	got := ResolveAccess("", []string{tmpDir}, "neutral")
	if got != "rw" {
		t.Errorf("expected fallback rw, got %q", got)
	}
}

// TestResolveAccess_FallbackOnInvalidValue verifies that invalid access
// values fall back to "rw" rather than propagating an unrecognized value.
func TestResolveAccess_FallbackOnInvalidValue(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithAccess(t, tmpDir, "broken", "writeonly")

	got := ResolveAccess("", []string{tmpDir}, "broken")
	if got != "rw" {
		t.Errorf("expected fallback rw on invalid value, got %q", got)
	}
}

// TestResolveAccess_TtalParentBlockEquivalent documents the parser limitation:
// the hand-rolled flat YAML parser does not distinguish ttal: vs lenos: parent
// blocks; both yield the same flat key "access". This is acceptable because
// the canonical home for the field is lenos.access, but legacy ttal.access
// blocks are functionally equivalent under this parser.
func TestResolveAccess_TtalParentBlockEquivalent(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "legacy")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Use ttal: parent block instead of lenos: — flat parser treats them the same.
	fm := "---\nname: legacy\nrole: reviewer\nttal:\n  access: ro\n---\n\n# Legacy\n"
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ResolveAccess("", []string{tmpDir}, "legacy")
	if got != "ro" {
		t.Errorf("expected ro (parser treats ttal/lenos blocks identically for flat keys), got %q", got)
	}
}

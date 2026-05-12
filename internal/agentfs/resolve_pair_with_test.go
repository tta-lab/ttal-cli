package agentfs

import (
	"os"
	"path/filepath"
	"testing"
)

func writeAgentWithPairWith(t *testing.T, dir, name, role, pairWith string) {
	t.Helper()
	agentDir := filepath.Join(dir, name)
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\nname: " + name + "\nrole: " + role + "\n"
	if pairWith != "" {
		fm += "lenos:\n  pair_with: " + pairWith + "\n"
	}
	fm += "---\n\n# " + name + "\n"
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePairWith_ReadsLenosPairWith(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithPairWith(t, tmpDir, "coder", "worker", "manager")

	got := ResolvePairWith("", []string{tmpDir}, "coder")
	if got != "manager" {
		t.Errorf("expected manager, got %q", got)
	}
}

func TestResolvePairWith_FallbackOnMissingAgent(t *testing.T) {
	tmpDir := t.TempDir()

	got := ResolvePairWith("", []string{tmpDir}, "ghost")
	if got != "" {
		t.Errorf("expected empty fallback, got %q", got)
	}
}

func TestResolvePairWith_FallbackOnEmptyPairWith(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithPairWith(t, tmpDir, "neutral", "worker", "")

	got := ResolvePairWith("", []string{tmpDir}, "neutral")
	if got != "" {
		t.Errorf("expected empty fallback, got %q", got)
	}
}

func TestResolvePairWith_ManagerRoleWithoutExplicitValueReturnsEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithPairWith(t, tmpDir, "any-manager", "manager", "")

	got := ResolvePairWith(tmpDir, nil, "any-manager")
	if got != "" {
		t.Errorf("expected empty pair target for manager without explicit frontmatter, got %q", got)
	}
}

func TestResolvePairWith_ExplicitValueReturnedForManager(t *testing.T) {
	tmpDir := t.TempDir()
	writeAgentWithPairWith(t, tmpDir, "custom-manager", "manager", "ops")

	got := ResolvePairWith(tmpDir, nil, "custom-manager")
	if got != "ops" {
		t.Errorf("expected explicit pair target ops, got %q", got)
	}
}

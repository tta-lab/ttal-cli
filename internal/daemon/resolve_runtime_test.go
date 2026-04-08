package daemon

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
)

// writeCoderAgent creates a {name}/AGENTS.md file in dir with the given default_runtime.
// An empty defaultRuntime omits the field from frontmatter.
func writeCoderAgent(t *testing.T, dir, defaultRuntime string) {
	t.Helper()
	agentDir := filepath.Join(dir, "coder")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	fm := "---\nname: coder\nrole: worker\n"
	if defaultRuntime != "" {
		fm += "default_runtime: " + defaultRuntime + "\n"
	}
	fm += "---\n\n# Coder\n"
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"), []byte(fm), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolveWorkerAgentRuntime_ReadsFrontmatter verifies that a valid
// default_runtime in frontmatter overrides the team default.
func TestResolveWorkerAgentRuntime_ReadsFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	writeCoderAgent(t, tmpDir, "lenos")

	got := resolveWorkerAgentRuntime("claude-code", "", []string{tmpDir}, "coder")
	if got != "lenos" {
		t.Errorf("expected %q, got %q", "lenos", got)
	}
}

// TestResolveWorkerAgentRuntime_FallbackOnMissingAgent verifies that when
// no agent is found in the search paths, the team default is preserved.
func TestResolveWorkerAgentRuntime_FallbackOnMissingAgent(t *testing.T) {
	tmpDir := t.TempDir() // empty — no agent subdirs

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	got := resolveWorkerAgentRuntime("claude-code", "", []string{tmpDir}, "coder")
	if got != "claude-code" {
		t.Errorf("expected %q (team default), got %q", "claude-code", got)
	}

	if !bytes.Contains(buf.Bytes(), []byte("no frontmatter for")) {
		t.Errorf("expected fallback log line containing 'no frontmatter for', got: %s", buf.String())
	}
}

// TestResolveWorkerAgentRuntime_FallbackOnEmptyDefaultRuntime verifies that
// when the agent has frontmatter but no default_runtime field, the team default
// is used and a log line is emitted.
func TestResolveWorkerAgentRuntime_FallbackOnEmptyDefaultRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	writeCoderAgent(t, tmpDir, "") // no default_runtime field

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	got := resolveWorkerAgentRuntime("claude-code", "", []string{tmpDir}, "coder")
	if got != "claude-code" {
		t.Errorf("expected %q (team default), got %q", "claude-code", got)
	}

	if !bytes.Contains(buf.Bytes(), []byte("no default_runtime")) {
		t.Errorf("expected log line containing 'no default_runtime', got: %s", buf.String())
	}
}

// TestResolveWorkerAgentRuntime_InvalidRuntime verifies that an unrecognized
// default_runtime value falls back to the team default with a warning log.
func TestResolveWorkerAgentRuntime_InvalidRuntime(t *testing.T) {
	tmpDir := t.TempDir()
	writeCoderAgent(t, tmpDir, "bogus-value")

	var buf bytes.Buffer
	orig := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(orig) })

	got := resolveWorkerAgentRuntime("claude-code", "", []string{tmpDir}, "coder")
	if got != "claude-code" {
		t.Errorf("expected %q (team default), got %q", "claude-code", got)
	}

	if !bytes.Contains(buf.Bytes(), []byte("invalid default_runtime")) {
		t.Errorf("expected log line containing 'invalid default_runtime', got: %s", buf.String())
	}
}

// TestResolveWorkerAgentRuntime_SecondPathWins verifies that ordered path
// search finds an agent in the second directory when absent from the first.
func TestResolveWorkerAgentRuntime_SecondPathWins(t *testing.T) {
	dir1 := t.TempDir() // no coder agent
	dir2 := t.TempDir()
	writeCoderAgent(t, dir2, "lenos")

	got := resolveWorkerAgentRuntime("claude-code", "", []string{dir1, dir2}, "coder")
	if got != "lenos" {
		t.Errorf("expected %q (from dir2 frontmatter), got %q", "lenos", got)
	}
}

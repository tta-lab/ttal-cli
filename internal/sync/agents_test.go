package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalCoderFrontmatter = `---
name: coder
emoji: ⚡
description: "Worker agent"
role: worker
color: green
default_runtime: lenos
---

# Coder

A test coder agent.
`

// TestDeployWorkerAgents_SubdirLayout verifies that DeployWorkerAgents reads
// agents from {name}/AGENTS.md subdirs, skips subdirs without AGENTS.md,
// and ignores top-level files.
func TestDeployWorkerAgents_SubdirLayout(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid agent subdir: coder/AGENTS.md
	coderDir := filepath.Join(tmpDir, "coder")
	if err := os.MkdirAll(coderDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(coderDir, "AGENTS.md"), []byte(minimalCoderFrontmatter), 0o644); err != nil {
		t.Fatal(err)
	}

	// Sibling subdir WITHOUT an AGENTS.md — must be skipped, not error
	noAgentDir := filepath.Join(tmpDir, "notanagent")
	if err := os.MkdirAll(noAgentDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Stray top-level README.md — must be ignored
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use dryRun=true to avoid writing to ~/.claude
	results, err := DeployWorkerAgents([]string{tmpDir}, true)
	if err != nil {
		t.Fatalf("DeployWorkerAgents: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}

	r := results[0]
	if r.Name != "coder" {
		t.Errorf("Name = %q, want %q", r.Name, "coder")
	}
	if !strings.HasSuffix(r.Source, "coder/AGENTS.md") {
		t.Errorf("Source = %q, expected to end in coder/AGENTS.md", r.Source)
	}
}

package pairing

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const (
	testCoder = "coder"
	testOwner = "astra"
)

func TestManagerUsesAdminHumanAlias(t *testing.T) {
	cfg := &config.Config{AdminHuman: &humanfs.Human{Alias: "neil", Admin: true}}

	got := Manager(cfg)
	if got != "neil" {
		t.Errorf("expected admin alias neil, got %q", got)
	}
}

func TestManagerMissingAdminReturnsEmpty(t *testing.T) {
	if got := Manager(&config.Config{}); got != "" {
		t.Errorf("expected empty pair target without admin human, got %q", got)
	}
	if got := Manager(nil); got != "" {
		t.Errorf("expected empty pair target without config, got %q", got)
	}
}

func TestPlanReviewerUsesTaskOwner(t *testing.T) {
	task := &taskwarrior.Task{Owner: testOwner}

	got := PlanReviewer(task)
	if got != testOwner {
		t.Errorf("expected task owner astra, got %q", got)
	}
}

func TestWorkerCoderUsesTaskOwner(t *testing.T) {
	task := &taskwarrior.Task{Owner: testOwner}

	got := Worker(&config.Config{}, testCoder, task)
	if got != testOwner {
		t.Errorf("expected coder to pair with task owner astra, got %q", got)
	}
}

func TestWorkerCoderWithoutOwnerReturnsEmpty(t *testing.T) {
	got := Worker(&config.Config{}, testCoder, &taskwarrior.Task{})
	if got != "" {
		t.Errorf("expected empty pair target without task owner, got %q", got)
	}
}

func TestReviewerPRReviewLeadPairsWithCoder(t *testing.T) {
	got := Reviewer(&config.Config{}, "pr-review-lead")
	if got != testCoder {
		t.Errorf("expected pr-review-lead to pair with coder, got %q", got)
	}
}

func TestCustomAgentUsesFrontmatter(t *testing.T) {
	tmpDir := t.TempDir()
	agentDir := filepath.Join(tmpDir, "custom-reviewer")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\nname: custom-reviewer\nlenos:\n  pair_with: coder\n---\n# Custom Reviewer\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Sync: config.SyncConfig{WorkerAgentPaths: []string{tmpDir}}}

	if got := Worker(cfg, "custom-reviewer", &taskwarrior.Task{Owner: testOwner}); got != testCoder {
		t.Errorf("expected custom worker to use frontmatter target coder, got %q", got)
	}
	if got := Reviewer(cfg, "custom-reviewer"); got != testCoder {
		t.Errorf("expected custom reviewer to use frontmatter target coder, got %q", got)
	}
}

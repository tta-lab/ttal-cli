package gitutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
)

const cmdTimeout = 10 * time.Second

// DumpWorkerState captures git state for debugging.
// Returns the path to the dump file.
func DumpWorkerState(sessionName, workDir, workerName string) (string, error) {
	dumpDir := filepath.Join(config.ResolveDataDir(), "dumps")
	if err := os.MkdirAll(dumpDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create dump directory: %w", err)
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dumpFile := filepath.Join(dumpDir, fmt.Sprintf("%s_%s.txt", workerName, timestamp))

	var sections []string
	sections = append(sections, fmt.Sprintf("Worker State Dump: %s", workerName))
	sections = append(sections, fmt.Sprintf("Timestamp: %s", timestamp))
	sections = append(sections, fmt.Sprintf("Session: %s", sessionName))
	sections = append(sections, fmt.Sprintf("Work dir: %s", workDir))
	sections = append(sections, "")

	if out, err := runGit(workDir, "log", "--oneline", "-20"); err == nil {
		sections = append(sections, "=== Recent commits ===", out)
	}

	if out, err := runGit(workDir, "status", "--short"); err == nil {
		sections = append(sections, "=== Git status ===", out)
	}

	if out, err := runGit(workDir, "log", "--oneline", "main..HEAD"); err == nil && strings.TrimSpace(out) != "" {
		sections = append(sections, "=== Commits not in main ===", out)
	}

	content := strings.Join(sections, "\n")
	if err := os.WriteFile(dumpFile, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("failed to write dump file: %w", err)
	}

	return dumpFile, nil
}

// IsWorktreeClean checks whether the worktree has uncommitted changes.
// Returns (true, nil) if clean, (false, nil) if dirty, or (false, err) if
// the git command itself failed (e.g. missing directory, timeout).
func IsWorktreeClean(workDir string) (bool, error) {
	out, err := runGit(workDir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status failed in %s: %w", workDir, err)
	}
	return strings.TrimSpace(out) == "", nil
}

// RemoveWorktree removes a git worktree and its branch.
// Uses os.RemoveAll + git worktree prune instead of git worktree remove
// for faster cleanup without subprocess timeout risks.
func RemoveWorktree(projectDir, workDir, branch string) error {
	// Remove worktree directory directly (faster than git worktree remove)
	if err := os.RemoveAll(workDir); err != nil {
		return fmt.Errorf("failed to remove worktree directory %s: %w", workDir, err)
	}

	// Prune stale worktree metadata so git knows it's gone
	if _, err := runGit(projectDir, "worktree", "prune"); err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	// Delete the worker branch
	if branch != "" {
		if _, err := runGit(projectDir, "branch", "-D", branch); err != nil {
			// Non-fatal: branch may already be deleted
			fmt.Fprintf(os.Stderr, "warning: failed to delete branch %s: %v\n", branch, err)
		}
	}

	return nil
}

// BranchName returns the current git branch for dir.
// Returns "" if in detached HEAD state or on any error.
func BranchName(dir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", dir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// GitCommonDir returns the path to the shared .git directory for a worktree.
// For a regular repo this returns "<repo>/.git". For a linked worktree it
// returns the main repo's ".git" directory (e.g. "/path/to/main/.git").
// Returns "" on any error (not a git repo, timeout, etc.).
func GitCommonDir(dir string) string {
	out, err := runGit(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return ""
	}
	p := strings.TrimSpace(out)
	if p == "" {
		return ""
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, p)
	}
	p = filepath.Clean(p)
	return p
}

// IsWorktreeLinked returns true if dir is a linked git worktree (not the main repo).
// Compares --git-dir (worktree-specific) against --git-common-dir (shared).
func IsWorktreeLinked(dir string) bool {
	gitDir, err := runGit(dir, "rev-parse", "--git-dir")
	if err != nil {
		return false
	}
	commonDir := GitCommonDir(dir)
	if commonDir == "" {
		return false
	}
	gd := strings.TrimSpace(gitDir)
	if !filepath.IsAbs(gd) {
		gd = filepath.Join(dir, gd)
	}
	gd = filepath.Clean(gd)
	return gd != commonDir
}

func runGit(dir string, args ...string) (string, error) {
	return runGitWithTimeout(cmdTimeout, dir, args...)
}

func runGitWithTimeout(timeout time.Duration, dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fullArgs := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

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

const (
	cmdTimeout            = 10 * time.Second
	worktreeRemoveTimeout = 120 * time.Second
)

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
func RemoveWorktree(projectDir, workDir, branch string) error {
	// Remove git worktree (must happen before branch deletion)
	if _, err := os.Stat(workDir); err == nil {
		if _, err := runGitWithTimeout(worktreeRemoveTimeout, projectDir, "worktree", "remove", workDir, "--force"); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check worktree directory %s: %w", workDir, err)
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

package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/forgejo"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// CloseResult holds the outcome of a close operation.
type CloseResult struct {
	Cleaned   bool
	Forced    bool
	Merged    string // "true", "false", "no_pr"
	Clean     bool
	Status    string
	StateDump string
	Error     bool
}

// Close handles worker cleanup with smart or force mode.
//
// Exit semantics (reflected in the returned error and CloseResult):
//
//	nil error + Cleaned=true  → cleaned up successfully (exit 0)
//	ErrNeedsDecision          → needs manual decision (exit 1)
//	other error               → script/worker error (exit 2)
func Close(sessionName string, force bool) (*CloseResult, error) {
	// Find the task by session_name (try completed first, then pending)
	task, err := taskwarrior.ExportTaskBySession(sessionName, taskStatusCompleted)
	if err != nil {
		task, err = taskwarrior.ExportTaskBySession(sessionName, taskStatusPending)
	}
	if err != nil {
		result := &CloseResult{
			Error:  true,
			Status: fmt.Sprintf("Task with session_name '%s' not found", sessionName),
		}
		return result, fmt.Errorf("task not found")
	}

	branch := task.Branch
	if branch == "" {
		return &CloseResult{Error: true, Status: "Task missing required UDA: branch"}, fmt.Errorf("missing branch UDA")
	}

	projectPath := task.ProjectPath
	if projectPath == "" {
		projectPath = "."
	}

	// Derive work_dir from branch name (worker/<name> → .worktrees/<name>)
	workerName := strings.TrimPrefix(branch, "worker/")
	workDir := filepath.Join(projectPath, ".worktrees", workerName)

	// Force mode: dump + cleanup + exit 0
	if force {
		dumpPath, _ := zellij.DumpSessionState(sessionName, workDir, sessionName)
		if err := zellij.CleanupWorker(sessionName, workDir, branch, projectPath); err != nil {
			return &CloseResult{
				Error:     true,
				Status:    "Worker cleanup failed",
				StateDump: dumpPath,
			}, fmt.Errorf("cleanup failed: %w", err)
		}
		pullMainBranch(projectPath)
		return &CloseResult{
			Cleaned:   true,
			Forced:    true,
			Status:    "Worker force-cleaned",
			StateDump: dumpPath,
		}, nil
	}

	// Smart mode: check PR + worktree
	if task.PRID == "" {
		// No pr_id — worker hasn't created a PR yet
		clean := zellij.CheckWorktreeClean(workDir)
		dumpPath, _ := zellij.DumpSessionState(sessionName, workDir, sessionName)
		return &CloseResult{
			Merged:    "no_pr",
			Clean:     clean,
			Status:    "Task completed but no PR created - check worker output",
			StateDump: dumpPath,
		}, ErrNeedsDecision
	}

	prID, err := strconv.ParseInt(task.PRID, 10, 64)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Invalid pr_id: %s", task.PRID)}, err
	}

	// Check if PR is merged
	owner, repo, err := forgejo.ParseRepoInfo(projectPath)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Could not detect repo info: %v", err)}, err
	}

	merged, err := forgejo.IsPRMerged(owner, repo, prID)
	if err != nil {
		return &CloseResult{
			Error:  true,
			Status: fmt.Sprintf("Failed to check PR #%s: %v", task.PRID, err),
		}, err
	}

	clean := zellij.CheckWorktreeClean(workDir)

	if !merged {
		dumpPath, _ := zellij.DumpSessionState(sessionName, workDir, sessionName)
		return &CloseResult{
			Merged:    "false",
			Clean:     clean,
			Status:    fmt.Sprintf("PR #%s not merged yet - worker needs review", task.PRID),
			StateDump: dumpPath,
		}, ErrNeedsDecision
	}

	// PR is merged
	if clean {
		// Safe to auto-cleanup
		if err := zellij.CleanupWorker(sessionName, workDir, branch, projectPath); err != nil {
			return &CloseResult{Error: true, Status: "Worker cleanup failed"}, fmt.Errorf("cleanup failed: %w", err)
		}
		pullMainBranch(projectPath)
		return &CloseResult{
			Cleaned: true,
			Status:  "Worker cleaned up (PR merged, worktree clean)",
		}, nil
	}

	// PR merged but worktree dirty
	dumpPath, _ := zellij.DumpSessionState(sessionName, workDir, sessionName)
	return &CloseResult{
		Merged:    "true",
		Clean:     false,
		Status:    "PR merged but worktree has uncommitted changes",
		StateDump: dumpPath,
	}, ErrNeedsDecision
}

// pullMainBranch pulls latest changes in the main project directory after cleanup.
func pullMainBranch(projectPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "pull", "--ff-only")
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: git pull in %s failed (non-fatal): %v\n", projectPath, err)
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "  output: %s\n", strings.TrimSpace(string(out)))
		}
	} else {
		fmt.Printf("Pulled latest changes in %s\n", projectPath)
	}
}

// ErrNeedsDecision indicates the worker needs manual intervention (exit code 1).
var ErrNeedsDecision = fmt.Errorf("needs manual decision")

// PrintResult outputs the close result in a machine-parseable format matching
// the Python script's output format.
func PrintResult(r *CloseResult) {
	if r.Error {
		fmt.Fprintf(os.Stderr, "error=true\n")
		fmt.Fprintf(os.Stderr, "status=%s\n", r.Status)
		return
	}

	if r.Cleaned {
		fmt.Println("cleaned=true")
		if r.Forced {
			fmt.Println("forced=true")
		}
	}
	if r.Merged != "" {
		fmt.Printf("merged=%s\n", r.Merged)
	}
	if r.Merged != "" || r.Cleaned {
		fmt.Printf("clean=%t\n", r.Clean)
	}
	fmt.Printf("status=%s\n", r.Status)
	if r.StateDump != "" {
		fmt.Printf("state_dump=%s\n", r.StateDump)
	}
}

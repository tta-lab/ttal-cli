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

	gitroot "codeberg.org/clawteam/ttal-cli/internal/git"
	"codeberg.org/clawteam/ttal-cli/internal/gitprovider"
	"codeberg.org/clawteam/ttal-cli/internal/gitutil"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
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
func Close(sessionID string, force bool) (*CloseResult, error) {
	// Find the task by UUID prefix (try completed first, then pending)
	// Handle both old (bare UUID[:8]) and new (w-UUID[:8]-slug) formats
	sid := taskwarrior.ExtractSessionID(sessionID)
	task, err := taskwarrior.ExportTaskBySessionID(sid, taskStatusCompleted)
	if err != nil {
		task, err = taskwarrior.ExportTaskBySessionID(sid, taskStatusPending)
	}
	if err != nil {
		result := &CloseResult{
			Error:  true,
			Status: fmt.Sprintf("Task with UUID prefix '%s' not found", sessionID),
		}
		return result, fmt.Errorf("task not found")
	}

	sessionName := task.SessionName()

	branch := task.Branch
	if branch == "" {
		return &CloseResult{Error: true, Status: "Task missing required UDA: branch"}, fmt.Errorf("missing branch UDA")
	}

	projectPath := task.ProjectPath
	if projectPath == "" {
		projectPath = "."
	}

	// Resolve git root — projectPath may be a subpath in a monorepo.
	// Worktrees and git pull must operate at the git root level.
	gitRoot, err := gitroot.FindRoot(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine git root for %s: %v\n", projectPath, err)
		gitRoot = projectPath
	}

	// Derive work_dir from branch name (worker/<name> → .worktrees/<name>)
	workerName := strings.TrimPrefix(branch, "worker/")
	workDir := filepath.Join(gitRoot, ".worktrees", workerName)

	// Force mode: dump + cleanup + exit 0
	if force {
		dumpPath := dumpState(sessionName, workDir)
		if err := cleanupWorker(sessionName, workDir, branch, gitRoot); err != nil {
			return &CloseResult{
				Error:     true,
				Status:    fmt.Sprintf("Worker cleanup failed: %v", err),
				StateDump: dumpPath,
			}, fmt.Errorf("cleanup failed: %w", err)
		}
		if task.UUID != "" {
			if err := taskwarrior.MarkDone(task.UUID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to mark task done %s: %v\n", task.UUID, err)
			}
		}
		pullMainBranch(gitRoot)
		archiveTaskPlans(task.UUID)
		return &CloseResult{
			Cleaned:   true,
			Forced:    true,
			Status:    "Worker force-cleaned",
			StateDump: dumpPath,
		}, nil
	}

	// If worktree directory is already gone, skip dirty checks entirely
	worktreeExists := dirExists(workDir)

	// Smart mode: check PR + worktree
	if task.PRID == "" {
		// No pr_id — worker hasn't created a PR yet
		clean := !worktreeExists // missing worktree = nothing to lose
		if worktreeExists {
			var cleanErr error
			clean, cleanErr = gitutil.IsWorktreeClean(workDir)
			if cleanErr != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", cleanErr)
			}
		}
		dumpPath := dumpState(sessionName, workDir)
		return &CloseResult{
			Merged:    "no_pr",
			Clean:     clean,
			Status:    "Task completed but no PR created - check worker output",
			StateDump: dumpPath,
		}, ErrNeedsDecision
	}

	return closeWithPR(task.UUID, task.PRID, gitRoot, sessionName, workDir, branch, worktreeExists)
}

// closeWithPR handles the smart-close path when a PR exists.
func closeWithPR(
	taskUUID, prIDStr, gitRoot, sessionName, workDir, branch string,
	worktreeExists bool,
) (*CloseResult, error) {
	prID, err := strconv.ParseInt(prIDStr, 10, 64)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Invalid pr_id: %s", prIDStr)}, err
	}

	info, err := gitprovider.DetectProvider(gitRoot)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Could not detect repo info: %v", err)}, err
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Could not create provider: %v", err)}, err
	}

	fetchedPR, err := provider.GetPR(info.Owner, info.Repo, prID)
	if err != nil {
		return &CloseResult{
			Error:  true,
			Status: fmt.Sprintf("Failed to check PR #%s: %v", prIDStr, err),
		}, err
	}

	clean := !worktreeExists // missing worktree = nothing to lose
	if worktreeExists {
		var cleanErr error
		clean, cleanErr = gitutil.IsWorktreeClean(workDir)
		if cleanErr != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", cleanErr)
		}
	}

	if !fetchedPR.Merged {
		dumpPath := dumpState(sessionName, workDir)
		return &CloseResult{
			Merged:    "false",
			Clean:     clean,
			Status:    fmt.Sprintf("PR #%s not merged yet - worker needs review", prIDStr),
			StateDump: dumpPath,
		}, ErrNeedsDecision
	}

	// PR is merged + worktree clean → auto-cleanup
	if clean {
		if err := cleanupWorker(sessionName, workDir, branch, gitRoot); err != nil {
			return &CloseResult{
				Error:  true,
				Status: fmt.Sprintf("Worker cleanup failed: %v", err),
			}, fmt.Errorf("cleanup failed: %w", err)
		}
		if taskUUID != "" {
			if err := taskwarrior.MarkDone(taskUUID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to mark task done %s: %v\n", taskUUID, err)
			}
		}
		pullMainBranch(gitRoot)
		archiveTaskPlans(taskUUID)
		return &CloseResult{
			Cleaned: true,
			Status:  "Worker cleaned up (PR merged, worktree clean)",
		}, nil
	}

	// PR merged but worktree dirty
	dumpPath := dumpState(sessionName, workDir)
	return &CloseResult{
		Merged:    "true",
		Clean:     false,
		Status:    "PR merged but worktree has uncommitted changes",
		StateDump: dumpPath,
	}, ErrNeedsDecision
}

// dumpState captures worker git state, logging any errors to stderr.
func dumpState(sessionName, workDir string) string {
	path, err := gitutil.DumpWorkerState(sessionName, workDir, sessionName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to dump worker state: %v\n", err)
	}
	return path
}

// cleanupWorker kills the tmux session and removes the git worktree + branch.
func cleanupWorker(sessionName, workDir, branch, gitRoot string) error {
	if tmux.SessionExists(sessionName) {
		if err := tmux.KillSession(sessionName); err != nil {
			return fmt.Errorf("failed to kill session: %w", err)
		}
	}

	return gitutil.RemoveWorktree(gitRoot, workDir, branch)
}

// archiveTaskPlans archives flicknote plan/design notes referenced in a task's
// annotations. Best-effort: failures are logged to stderr but never returned.
// Called after successful cleanup so plan notes don't linger after PR merges.
func archiveTaskPlans(taskUUID string) {
	if taskUUID == "" {
		return
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return
	}

	for _, ann := range task.Annotations {
		m := taskwarrior.HexIDPattern.FindStringSubmatch(ann.Description)
		if len(m) == 0 {
			continue
		}
		hexID := m[1]

		note := taskwarrior.ReadFlicknoteJSON(hexID)
		if note == nil {
			continue
		}
		if !taskwarrior.ShouldInlineNote(note) {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, "flicknote", "archive", hexID)
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to archive flicknote %s: %v\n", hexID, err)
		} else {
			fmt.Printf("Archived plan note: %s\n", hexID)
		}
		cancel()
	}
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

// dirExists returns true if path exists and is a directory.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: cannot stat worktree %s: %v (treating as missing)\n", path, err)
		}
		return false
	}
	return info.IsDir()
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

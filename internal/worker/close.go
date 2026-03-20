package worker

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	gitroot "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
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
// team is the TTAL team name from the cleanup request; pass "" when calling
// from the CLI (uses the active team from config).
//
// Exit semantics (reflected in the returned error and CloseResult):
//
//	nil error + Cleaned=true  → cleaned up successfully (exit 0)
//	ErrNeedsDecision          → needs manual decision (exit 1)
//	other error               → script/worker error (exit 2)
func Close(sessionID string, force bool, team string) (*CloseResult, error) {
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

	// Derive work_dir from task UUID and project alias
	workDir := filepath.Join(config.WorktreesRoot(), fmt.Sprintf("%s-%s", task.UUID[:8], task.Project))

	// Compute branch at runtime from the worktree (no branch UDA needed)
	branch := gitutil.BranchName(workDir)
	if branch == "" {
		return &CloseResult{Error: true, Status: "No active worktree found for this task"}, fmt.Errorf("no worktree branch found at %s", workDir)
	}

	// Use team-aware resolution so the daemon (which caches config via sync.Once at startup)
	// reads the correct team's projects.toml rather than its own.
	projectPath := project.ResolveProjectPathForTeam(task.Project, team)
	if projectPath == "" {
		fmt.Fprintf(os.Stderr, "warning: project %q not found in projects.toml\n", task.Project)
		return closeWithoutProject(task, sessionName, workDir)
	}

	// Resolve git root — projectPath may be a subpath in a monorepo.
	// Worktrees and git pull must operate at the git root level.
	gitRoot, err := gitroot.FindRoot(projectPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not determine git root for %s: %v\n", projectPath, err)
		gitRoot = projectPath
	}

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
		archiveTaskPlans(task.Annotations)
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

	return closeWithPR(task.UUID, task.PRID, gitRoot, sessionName, workDir, branch, worktreeExists, task.Annotations)
}

// closeWithoutProject handles cleanup when the project alias can't be resolved in projects.toml.
// Cleanup requests are only created after successful merge, so the PR is already merged.
// Performs best-effort cleanup: kills the tmux session, removes the worktree directory,
// and marks the task done — skipping git operations that require a valid repo root.
func closeWithoutProject(task *taskwarrior.Task, sessionName, workDir string) (*CloseResult, error) {
	if tmux.SessionExists(sessionName) {
		if err := tmux.KillSession(sessionName); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to kill session %s: %v\n", sessionName, err)
		}
	}
	if err := os.RemoveAll(workDir); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to remove worktree dir %s: %v\n", workDir, err)
	}
	// Skip git worktree prune + branch delete — no valid gitRoot.
	// Orphaned metadata is cleaned up on next manual `git worktree prune` in the real repo.
	if err := taskwarrior.MarkDone(task.UUID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to mark task done %s: %v\n", task.UUID, err)
	}
	archiveTaskPlans(task.Annotations)
	return &CloseResult{
		Cleaned: true,
		Status:  fmt.Sprintf("Worker cleaned up (project %q unresolvable — skipped PR check and git cleanup)", task.Project),
	}, nil
}

// closeWithPR handles the smart-close path when a PR exists.
func closeWithPR(
	taskUUID, prIDStr, gitRoot, sessionName, workDir, branch string,
	worktreeExists bool, annotations []taskwarrior.Annotation,
) (*CloseResult, error) {
	pridInfo, err := taskwarrior.ParsePRID(prIDStr)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Invalid pr_id: %s", prIDStr)}, err
	}
	prID := pridInfo.Index

	repoInfo, err := gitprovider.DetectProvider(gitRoot)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Could not detect repo info: %v", err)}, err
	}

	provider, err := gitprovider.NewProvider(repoInfo)
	if err != nil {
		return &CloseResult{Error: true, Status: fmt.Sprintf("Could not create provider: %v", err)}, err
	}

	fetchedPR, err := provider.GetPR(repoInfo.Owner, repoInfo.Repo, prID)
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
		archiveTaskPlans(annotations)
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
// annotations. Best-effort: failures are logged but never returned.
// Called after successful cleanup so plan notes don't linger after PR merges.
func archiveTaskPlans(annotations []taskwarrior.Annotation) {
	if len(annotations) == 0 {
		return
	}

	inlineProjects := taskwarrior.LoadInlineProjects()

	for _, ann := range annotations {
		m := taskwarrior.HexIDPattern.FindStringSubmatch(ann.Description)
		if len(m) == 0 {
			continue
		}
		hexID := m[1]

		note := taskwarrior.ReadFlicknoteJSON(hexID)
		if note == nil {
			log.Printf("[archive] flicknote %s not found or not readable — skipping", hexID)
			continue
		}
		if !taskwarrior.ShouldInlineNote(note, inlineProjects) {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, "flicknote", "archive", hexID)
		if err := cmd.Run(); err != nil {
			log.Printf("[archive] warning: failed to archive flicknote %s: %v", hexID, err)
		} else {
			log.Printf("[archive] archived plan note: %s", hexID)
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

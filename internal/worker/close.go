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
	"github.com/tta-lab/ttal-cli/internal/temenos"
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
//
// Exit semantics (reflected in the returned error and CloseResult):
//
//	nil error + Cleaned=true  → cleaned up successfully (exit 0)
//	ErrNeedsDecision          → needs manual decision (exit 1)
//	other error               → script/worker error (exit 2)
func Close(sessionID string, force bool) (*CloseResult, error) {
	// Find the task by UUID prefix (try completed first, then pending)
	// Handle both old (bare UUID[:8]) and new (w-UUID[:8]-slug) formats
	sid := taskwarrior.ExtractHexID(sessionID)
	task, err := taskwarrior.ExportTaskByHexID(sid, taskStatusCompleted)
	if err != nil {
		task, err = taskwarrior.ExportTaskByHexID(sid, taskStatusPending)
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
		return &CloseResult{Error: true, Status: "No active worktree found for this task"},
			fmt.Errorf("no worktree branch found at %s", workDir)
	}

	projectPath := project.ResolveProjectPath(task.Project)
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
		if err := cleanupWorker(sessionName, workDir, branch, gitRoot, task.HexID(), task.Annotations); err != nil {
			return &CloseResult{
				Error:     true,
				Status:    fmt.Sprintf("Worker cleanup failed: %v", err),
				StateDump: dumpPath,
			}, fmt.Errorf("cleanup failed: %w", err)
		}
		if task.UUID != "" {
			// Force close bypasses the normal pipeline completion path. If the on-modify
			// hook blocks completion (last stage missing _lgtm tag), manually add the
			// stage tag: task <uuid> modify +<laststage>_lgtm && task <uuid> done
			if err := taskwarrior.MarkDone(task.UUID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: failed to mark task done %s: %v\n", task.UUID, err)
			}
		}
		pullMainBranch(gitRoot, task.Project)
		deleteTaskPlans(task.Annotations)
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

	return closeWithPR(
		task.UUID, task.PRID, task.Project,
		gitRoot, sessionName, workDir, branch,
		worktreeExists, task.Annotations,
	)
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
	deleteTaskPlans(task.Annotations)
	return &CloseResult{
		Cleaned: true,
		Status:  fmt.Sprintf("Worker cleaned up (project %q unresolvable — skipped PR check and git cleanup)", task.Project),
	}, nil
}

// closeWithPR handles the smart-close path when a PR exists.
func closeWithPR(
	taskUUID, prIDStr, projectAlias, gitRoot, sessionName, workDir, branch string,
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

	token := project.ResolveGitHubToken(projectAlias)
	provider, err := gitprovider.NewProviderWithToken(repoInfo, token)
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
		hexID := taskUUID
		if len(hexID) >= 8 {
			hexID = hexID[:8]
		}
		if err := cleanupWorker(sessionName, workDir, branch, gitRoot, hexID, annotations); err != nil {
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
		pullMainBranch(gitRoot, projectAlias)
		deleteTaskPlans(annotations)
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
// taskHexID is used to delete the worker's MCP config file (~/.ttal/mcps/w-<hexid>.json).
// annotations are used to extract temenos session tokens for cleanup (best-effort).
// Cleans up both the worker token and any reviewer tokens (PR and plan).
func cleanupWorker(
	sessionName, workDir, branch, gitRoot, taskHexID string,
	annotations []taskwarrior.Annotation,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Delete worker temenos session — best-effort, 8h TTL handles expiry on error.
	if token := temenos.ExtractToken(annotations); token != "" {
		if err := temenos.DeleteSessionByToken(ctx, token); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete temenos session (non-fatal): %v\n", err)
		}
	}
	// Delete worker MCP config file.
	if taskHexID != "" {
		temenos.DeleteMCPConfigFile("w-" + taskHexID)
	}

	// Delete reviewer temenos sessions (best-effort).
	cleanupReviewerTokens(ctx, taskHexID, annotations)

	if tmux.SessionExists(sessionName) {
		if err := tmux.KillSession(sessionName); err != nil {
			return fmt.Errorf("failed to kill session: %w", err)
		}
	}

	return gitutil.RemoveWorktree(gitRoot, workDir, branch)
}

// cleanupReviewerTokens scans annotations for reviewer temenos tokens and deletes them.
// Handles both PR reviewer (temenos_pr_reviewer_token) and plan reviewer (temenos_plan_reviewer_token).
func cleanupReviewerTokens(ctx context.Context, taskHexID string, annotations []taskwarrior.Annotation) {
	const (
		prReviewerPrefix   = "temenos_pr_reviewer_token:"
		planReviewerPrefix = "temenos_plan_reviewer_token:"
	)

	for _, ann := range annotations {
		var token, mcpName string
		switch {
		case strings.HasPrefix(ann.Description, prReviewerPrefix):
			token = strings.TrimPrefix(ann.Description, prReviewerPrefix)
			mcpName = temenos.ReviewerMCPName(taskHexID, "pr")
		case strings.HasPrefix(ann.Description, planReviewerPrefix):
			token = strings.TrimPrefix(ann.Description, planReviewerPrefix)
			mcpName = temenos.ReviewerMCPName(taskHexID, "plan")
		default:
			continue
		}
		if token == "" {
			continue
		}
		if err := temenos.DeleteSessionByToken(ctx, token); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to delete reviewer temenos session (non-fatal): %v\n", err)
		}
		temenos.DeleteMCPConfigFile(mcpName)
	}
}

// deleteTaskPlans deletes flicknote plan/design notes referenced in a task's
// annotations. Best-effort: failures are logged but never returned.
// Called after successful cleanup so plan notes don't linger after PR merges.
func deleteTaskPlans(annotations []taskwarrior.Annotation) {
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
		cmd := exec.CommandContext(ctx, "flicknote", "delete", hexID)
		if err := cmd.Run(); err != nil {
			log.Printf("[archive] warning: failed to delete flicknote %s: %v", hexID, err)
		} else {
			log.Printf("[archive] deleted plan note: %s", hexID)
		}
		cancel()
	}
}

// pullMainBranch pulls latest changes in the main project directory after cleanup.
func pullMainBranch(projectPath, projectAlias string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", projectPath, "pull", "--ff-only")

	remoteURL, remoteErr := gitutil.RemoteURL(projectPath)
	if remoteErr != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not get remote URL, pull runs without credentials: %v\n", remoteErr)
	} else {
		cmd.Env = append(os.Environ(), gitutil.GitCredEnv(remoteURL, projectAlias)...)
	}

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

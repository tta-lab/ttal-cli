package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// SpawnConfig holds configuration for spawning a worker.
type SpawnConfig struct {
	Name     string
	Project  string
	TaskUUID string
	Worktree bool
	Force    bool
	Yolo     bool
}

// Spawn creates a new worker: validates task, sets up worktree, launches zellij session,
// and tracks the worker in taskwarrior.
func Spawn(cfg SpawnConfig) error {
	task, err := loadAndValidateTask(cfg)
	if err != nil {
		return err
	}

	project, err := filepath.Abs(cfg.Project)
	if err != nil {
		return fmt.Errorf("failed to resolve project path: %w", err)
	}
	if _, err := os.Stat(project); err != nil {
		return fmt.Errorf("project directory not found: %s", project)
	}

	sessionName := task.SessionName()

	fmt.Printf("Spawning CC worker: %s\n  Project: %s\n  Task: %s\n\n", cfg.Name, project, task.Description)
	fmt.Printf("Creating zellij session: %s (from task UUID for worker '%s')\n", sessionName, cfg.Name)

	if err := ensureSessionAvailable(cfg, sessionName, project); err != nil {
		return err
	}

	workDir, branch, err := setupWorkDir(cfg, project)
	if err != nil {
		return err
	}

	return launchAndTrack(cfg, task, sessionName, workDir, branch, project)
}

func loadAndValidateTask(cfg SpawnConfig) (*taskwarrior.Task, error) {
	if err := taskwarrior.ValidateUUID(cfg.TaskUUID); err != nil {
		return nil, err
	}

	task, err := taskwarrior.ExportTask(strings.TrimSpace(cfg.TaskUUID))
	if err != nil {
		return nil, err
	}
	fmt.Printf("Loaded task from taskwarrior\n  UUID: %s\n", task.UUID)

	if err := taskwarrior.VerifyRequiredUDAs(); err != nil {
		return nil, err
	}

	return task, nil
}

func ensureSessionAvailable(cfg SpawnConfig, sessionName, project string) error {
	if !zellij.SessionExists(sessionName) {
		return nil
	}

	if cfg.Force {
		fmt.Printf("Session '%s' exists, closing it (--force)\n", sessionName)
		return zellij.DeleteSession(sessionName)
	}

	return fmt.Errorf("session '%s' already exists\n"+
		"  Worker '%s' in project '%s' is already running\n"+
		"  Use --force to respawn",
		sessionName, cfg.Name, filepath.Base(project))
}

func setupWorkDir(cfg SpawnConfig, project string) (workDir, branch string, err error) {
	if cfg.Worktree {
		workDir, err = setupWorktree(project, cfg.Name)
		if err != nil {
			return "", "", fmt.Errorf("failed to setup worktree: %w", err)
		}
		return workDir, fmt.Sprintf("worker/%s", cfg.Name), nil
	}

	fmt.Println("Working in main directory (no worktree)")
	return project, detectBranch(project), nil
}

func launchAndTrack(cfg SpawnConfig, task *taskwarrior.Task, sessionName, workDir, branch, project string) error {
	model := "opus"
	if task.HasTag("sonnet") {
		model = "sonnet"
	}

	fmt.Printf("\nLaunching Claude Code with task: %s\n", task.Description)
	fmt.Printf("  Model: %s\n", model)
	layoutFile, _, err := zellij.CreateLayout(zellij.LayoutConfig{
		WorkDir:    workDir,
		Task:       task.FormatPrompt(),
		Yolo:       cfg.Yolo,
		Brainstorm: task.HasTag("brainstorm"),
		Model:      model,
		Branch:     branch,
		IsWorktree: cfg.Worktree,
	})
	if err != nil {
		return fmt.Errorf("failed to create layout: %w", err)
	}

	handle, err := zellij.LaunchSession(sessionName, layoutFile)
	if err != nil {
		return err
	}

	if err := zellij.WaitForSession(sessionName, handle, 30*time.Second); err != nil {
		return fmt.Errorf("failed to create zellij session\n"+
			"  Session name: %s\n"+
			"  Worker: %s\n\n"+
			"  %w", sessionName, cfg.Name, err)
	}

	if err := taskwarrior.UpdateWorkerMetadata(task.UUID, branch, project); err != nil {
		return fmt.Errorf("worker spawn incomplete - session created but task tracking failed\n"+
			"  Session: %s\n"+
			"  To attach: zellij --data-dir %s attach %s\n\n"+
			"  %w", sessionName, zellij.DataDir(), sessionName, err)
	}

	fmt.Printf("\nWorker '%s' spawned successfully\n", cfg.Name)
	fmt.Printf("  Session: %s\n", sessionName)
	fmt.Printf("  Work dir: %s\n", workDir)
	fmt.Printf("\nTo attach:\n")
	fmt.Printf("  zellij --data-dir %s attach %s\n", zellij.DataDir(), sessionName)

	return nil
}

func setupWorktree(project, name string) (string, error) {
	worktreeDir := filepath.Join(project, ".worktrees", name)
	workerBranch := fmt.Sprintf("worker/%s", name)

	// Reuse existing worktree
	if info, err := os.Stat(worktreeDir); err == nil && info.IsDir() {
		fmt.Printf("Worktree already exists at %s, reusing\n", worktreeDir)
		return worktreeDir, nil
	}

	// Pull latest from remote so worktree branches from up-to-date main
	pullLatest(project)

	if err := createWorktree(project, worktreeDir, workerBranch); err != nil {
		return "", err
	}

	runWorktreeSetup(worktreeDir)
	return worktreeDir, nil
}

func createWorktree(project, worktreeDir, workerBranch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", project, "branch", "--list", workerBranch)
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check branch: %w", err)
	}

	if strings.Contains(string(out), workerBranch) {
		fmt.Printf("Branch '%s' exists, creating worktree on it\n", workerBranch)
		cmd = exec.CommandContext(ctx, "git", "-C", project, "worktree", "add", "--force", worktreeDir, workerBranch)
	} else {
		fmt.Printf("Creating new branch '%s'\n", workerBranch)
		cmd = exec.CommandContext(ctx, "git", "-C", project, "worktree", "add", "-b", workerBranch, worktreeDir)
	}

	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create worktree: %w: %s", err, strings.TrimSpace(string(out)))
	}

	return nil
}

func runWorktreeSetup(worktreeDir string) {
	setupScript := filepath.Join(worktreeDir, ".worktree-setup")
	info, err := os.Stat(setupScript)
	if err != nil || info.IsDir() {
		return
	}

	fmt.Println("\nRunning .worktree-setup...")
	if err := os.Chmod(setupScript, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: failed to make .worktree-setup executable: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Run through fish to get proper PATH (proto, moon, bun, etc.)
	// matching how the zellij session itself runs via fish -C
	cmd := exec.CommandContext(ctx, "fish", "-c", fmt.Sprintf("cd %s && ./.worktree-setup", worktreeDir))
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: .worktree-setup failed (non-fatal): %v\n", err)
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "  output: %s\n", strings.TrimSpace(string(out)))
		}
	} else {
		fmt.Println("  .worktree-setup completed successfully")
	}
}

func pullLatest(project string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Pulling latest changes...")
	cmd := exec.CommandContext(ctx, "git", "-C", project, "pull", "--ff-only")
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: git pull failed (non-fatal): %v\n", err)
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "  output: %s\n", strings.TrimSpace(string(out)))
		}
	}
}

func detectBranch(workDir string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", workDir, "branch", "--show-current")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	if b := strings.TrimSpace(string(out)); b != "" {
		return b
	}
	return "unknown"
}

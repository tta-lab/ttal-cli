package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// SpawnConfig holds configuration for spawning a worker.
type SpawnConfig struct {
	Name     string
	Project  string
	TaskUUID string
	Worktree bool
	Force    bool
	Yolo     bool
	Runtime  Runtime
}

// Spawn creates a new worker: validates task, sets up worktree, launches tmux session,
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

	// Route by task tag: +opencode or +oc overrides default runtime
	if cfg.Runtime == "" || cfg.Runtime == RuntimeClaudeCode {
		if task.HasTag("opencode") || task.HasTag("oc") {
			cfg.Runtime = RuntimeOpenCode
		}
	}
	if cfg.Runtime == "" {
		cfg.Runtime = RuntimeClaudeCode
	}

	if err := validateRuntime(cfg.Runtime); err != nil {
		return err
	}

	sessionName := task.SessionName()

	fmt.Printf("Spawning %s worker: %s\n  Project: %s\n  Task: %s\n\n", cfg.Runtime, cfg.Name, project, task.Description)
	fmt.Printf("Creating tmux session: %s (from task UUID for worker '%s')\n", sessionName, cfg.Name)

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
	if !tmux.SessionExists(sessionName) {
		return nil
	}

	if cfg.Force {
		fmt.Printf("Session '%s' exists, closing it (--force)\n", sessionName)
		return tmux.KillSession(sessionName)
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
	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	taskFile, err := writeTaskFile(task, cfg, workDir, branch)
	if err != nil {
		return err
	}

	shellCfg, _ := config.Load()
	if shellCfg == nil {
		shellCfg = &config.Config{}
	}

	taskrc := resolveTaskRCFromConfig(shellCfg)
	envParts := buildEnvParts(task, taskrc)
	shellCmd := buildLaunchCmd(cfg, ttalBin, taskFile, task, envParts, shellCfg)

	fmt.Printf("\nLaunching %s with task: %s\n", cfg.Runtime, task.Description)

	if err := tmux.NewSession(sessionName, "worker", workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Set env vars at session level so new windows/panes inherit them
	if err := tmux.SetEnv(sessionName, "TTAL_JOB_ID", task.SessionID()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to set session env: %v\n", err)
	}
	if team := os.Getenv("TTAL_TEAM"); team != "" {
		_ = tmux.SetEnv(sessionName, "TTAL_TEAM", team)
	}
	if taskrc != "" {
		_ = tmux.SetEnv(sessionName, "TASKRC", taskrc)
	}
	// Set OPENCODE_PERMISSION via tmux env to avoid shell quoting issues with JSON
	if cfg.Runtime == RuntimeOpenCode && cfg.Yolo {
		_ = tmux.SetEnv(sessionName, "OPENCODE_PERMISSION",
			`{"bash":"allow","edit":"allow","read":"allow","write":"allow","question":"allow"}`)
	}

	if err := taskwarrior.UpdateWorkerMetadata(task.UUID, branch, project); err != nil {
		return fmt.Errorf("session created but task tracking failed\n"+
			"  Session: %s\n"+
			"  To attach: tmux attach -t %s\n\n"+
			"  %w", sessionName, sessionName, err)
	}

	fmt.Printf("\nWorker '%s' spawned successfully\n", cfg.Name)
	fmt.Printf("  Session: %s\n", sessionName)
	fmt.Printf("  Work dir: %s\n", workDir)
	fmt.Printf("\nTo attach:\n")
	fmt.Printf("  tmux attach -t %s\n", sessionName)

	return nil
}

// buildEnvParts returns the shared env vars for any runtime.
func buildEnvParts(task *taskwarrior.Task, taskrc string) []string {
	parts := []string{fmt.Sprintf("TTAL_JOB_ID=%s", task.SessionID())}
	if team := os.Getenv("TTAL_TEAM"); team != "" {
		parts = append(parts, fmt.Sprintf("TTAL_TEAM=%s", team))
	}
	if taskrc != "" {
		parts = append(parts, fmt.Sprintf("TASKRC=%s", taskrc))
	}
	return parts
}

func buildLaunchCmd(cfg SpawnConfig, ttalBin, taskFile string, task *taskwarrior.Task,
	envParts []string, shellCfg *config.Config,
) string {
	switch cfg.Runtime {
	case RuntimeOpenCode:
		return buildOpenCodeCmd(ttalBin, taskFile, envParts, shellCfg)
	default:
		return buildClaudeCodeCmd(cfg, ttalBin, taskFile, task, envParts, shellCfg)
	}
}

func buildClaudeCodeCmd(cfg SpawnConfig, ttalBin, taskFile string, task *taskwarrior.Task,
	envParts []string, shellCfg *config.Config,
) string {
	model := "opus"
	if task.HasTag("sonnet") {
		model = "sonnet"
	}
	fmt.Printf("  Model: %s\n", model)

	yoloFlag := ""
	if cfg.Yolo {
		yoloFlag = "--dangerously-skip-permissions "
	}

	claudeCmd := fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- claude --model %s %s--",
		ttalBin, taskFile, model, yoloFlag)

	return shellCfg.BuildEnvShellCommand(envParts, claudeCmd)
}

func buildOpenCodeCmd(ttalBin, taskFile string, envParts []string, shellCfg *config.Config) string {
	ocCmd := fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- opencode --prompt",
		ttalBin, taskFile)

	return shellCfg.BuildEnvShellCommand(envParts, ocCmd)
}

func writeTaskFile(task *taskwarrior.Task, cfg SpawnConfig, workDir, branch string) (string, error) {
	fullTask := task.FormatPrompt()

	if cfg.Worktree && branch != "" {
		worktreePrefix := fmt.Sprintf("IMPORTANT - You are in a git worktree:\n"+
			"- Working directory: %s\n"+
			"- Branch: %s\n"+
			"- This is an isolated workspace for your task\n"+
			"- STAY in this directory - do NOT cd to parent/main workspace\n"+
			"- All your work should happen here\n"+
			"- When done: commit, push, and create PR with `ttal pr create \"title\" --body \"description\"`\n"+
			"\nYour task:\n\n", workDir, branch)
		fullTask = worktreePrefix + fullTask
	}

	if task.HasTag("brainstorm") {
		brainstormPrefix := `Use the brainstorming skill before implementation:

1. Understand the project context (check files, docs, recent commits)
2. Ask clarifying questions one at a time to refine requirements
3. Explore different approaches with trade-offs
4. Present the design in sections, validating each part
5. Document the design in docs/plans/YYYY-MM-DD-<topic>-design.md

Then proceed with:

`
		fullTask = brainstormPrefix + fullTask
	}

	taskFile, err := os.CreateTemp("", "claude-task-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create task file: %w", err)
	}
	if _, err := taskFile.WriteString(fullTask); err != nil {
		_ = taskFile.Close()
		return "", fmt.Errorf("failed to write task file: %w", err)
	}
	_ = taskFile.Close()
	return taskFile.Name(), nil
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

	cmd := exec.CommandContext(ctx, setupScript)
	cmd.Dir = worktreeDir
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

// resolveTaskRCFromConfig returns the taskrc path from the provided config.
// Returns empty string if using default taskrc.
func resolveTaskRCFromConfig(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	taskrc := cfg.TaskRC()
	if taskrc == config.DefaultTaskRC() {
		return ""
	}
	return taskrc
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

package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/env"
	git "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// SpawnConfig holds configuration for spawning a worker.
type SpawnConfig struct {
	Name     string
	Project  string
	TaskUUID string
	Worktree bool
	Force    bool
	Runtime  runtime.Runtime
	Spawner  string // team:agent format, set by ttal go
}

// Spawn creates a new worker: validates task, sets up worktree, launches tmux session,
// and tracks the worker in taskwarrior.
func Spawn(cfg SpawnConfig) error {
	return spawnWorker(cfg)
}

// spawnWorker spawns a tmux worker.
func spawnWorker(cfg SpawnConfig) error {
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

	// Detect git root — project may be a subpath within a monorepo
	gitRoot, err := git.FindRoot(project)
	if err != nil {
		return fmt.Errorf("cannot find git root for %s: %w", project, err)
	}

	// Compute relative subpath from git root to project directory.
	subpath, err := computeSubpath(project, gitRoot)
	if err != nil {
		return err
	}

	cfg.Runtime = resolveRuntime(cfg.Runtime, task)
	if err := validateRuntime(cfg.Runtime); err != nil {
		return err
	}

	sessionName := task.SessionName()

	fmt.Printf("Spawning %s worker: %s\n  Project: %s\n  Task: %s\n\n", cfg.Runtime, cfg.Name, project, task.Description)
	fmt.Printf("Creating tmux session: %s (from task UUID for worker '%s')\n", sessionName, cfg.Name)

	if err := ensureSessionAvailable(cfg, sessionName, project); err != nil {
		return err
	}

	workDir, branch, err := setupWorkDir(cfg, task, gitRoot)
	if err != nil {
		return err
	}

	// Adjust workDir for monorepo subpath
	worktreeRoot := workDir
	if subpath != "" {
		workDir = filepath.Join(workDir, subpath)
		if _, err := os.Stat(workDir); err != nil {
			return fmt.Errorf("subpath %s does not exist in worktree: %w", subpath, err)
		}
	}

	// Run .worktree-setup: subpath's script takes priority, then fall back to root.
	// Runs on both fresh and reused worktrees — setup scripts should be idempotent
	// (e.g. bun install, npm ci) so re-running is safe and keeps deps up to date.
	if cfg.Worktree {
		runWorktreeSetupWithFallback(workDir, worktreeRoot)
	}

	return launchTmuxWorker(cfg, task, sessionName, workDir, branch)
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

// computeSubpath computes the relative subpath from git root to project directory.
// Resolves symlinks before comparing — git rev-parse resolves them but filepath.Abs does not.
func computeSubpath(project, gitRoot string) (string, error) {
	resolvedProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to resolve symlinks for project: %v\n", err)
		resolvedProject = project
	}
	resolvedRoot, err := filepath.EvalSymlinks(gitRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to resolve symlinks for git root: %v\n", err)
		resolvedRoot = gitRoot
	}
	if resolvedProject != resolvedRoot {
		rel, err := filepath.Rel(gitRoot, project)
		if err != nil {
			return "", fmt.Errorf("cannot compute relative subpath: %w", err)
		}
		fmt.Printf("  Monorepo subpath: %s\n", rel)
		return rel, nil
	}
	return "", nil
}

// resolveModel determines the worker model: +hard tag uses opus, otherwise team worker_model config.
func resolveModel(task *taskwarrior.Task, shellCfg *config.Config) string {
	if task.HasTag("hard") {
		return "opus"
	}
	return shellCfg.WorkerModel()
}

// resolveRuntime determines the worker runtime from config, defaulting to ClaudeCode.
func resolveRuntime(rt runtime.Runtime, task *taskwarrior.Task) runtime.Runtime {
	if rt == "" || rt == runtime.ClaudeCode {
		if task.HasTag("codex") || task.HasTag("cx") {
			return runtime.Codex
		}
	}
	if rt != "" {
		return rt
	}
	shellCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load config: %v\n", err)
	}
	if shellCfg != nil {
		return shellCfg.WorkerRuntime()
	}
	return runtime.ClaudeCode
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

func setupWorkDir(cfg SpawnConfig, task *taskwarrior.Task, project string) (workDir, branch string, err error) {
	if cfg.Worktree {
		workDir, err = setupWorktree(project, task.SessionID(), cfg.Name, task.Project)
		if err != nil {
			return "", "", fmt.Errorf("failed to setup worktree: %w", err)
		}
		return workDir, fmt.Sprintf("worker/%s", cfg.Name), nil
	}

	fmt.Println("Working in main directory (no worktree)")
	return project, detectBranch(project), nil
}

// launchTmuxWorker spawns a worker in a tmux session.
func launchTmuxWorker(cfg SpawnConfig, task *taskwarrior.Task, sessionName, workDir, branch string) error {
	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	shellCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	taskrc := resolveTaskRCFromConfig(shellCfg)
	envParts := buildEnvParts(task, cfg.Runtime, taskrc)
	model := resolveModel(task, shellCfg)

	var shellCmd string
	var ccSessionPath string // non-empty for CC workers; cleaned up if tmux.NewSession fails

	if cfg.Runtime == runtime.Codex {
		// Codex workers stay on the old task-file path until #321
		taskFile, err := writeTaskFile(task, cfg, shellCfg)
		if err != nil {
			return err
		}
		codexCmd, err := launchcmd.BuildCodexGatekeeperCommand(ttalBin, taskFile)
		if err != nil {
			return err
		}
		shellCmd = shellCfg.BuildEnvShellCommand(envParts, codexCmd)
	} else {
		// Claude Code: JSONL session + trigger
		systemPrompt, err := writeSessionPrompt(task, cfg, shellCfg)
		if err != nil {
			return err
		}
		sessionPath, resumeCmd, err := launchcmd.BuildCCSessionCommand(
			ttalBin, workDir, breathe.SessionConfig{
				CWD:       workDir,
				GitBranch: branch,
				Handoff:   systemPrompt,
			}, model, "coder", "Begin implementation.",
		)
		if err != nil {
			return err
		}
		ccSessionPath = sessionPath
		shellCmd = shellCfg.BuildEnvShellCommand(envParts, resumeCmd)
	}

	fmt.Printf("\nLaunching %s with task: %s\n", cfg.Runtime, task.Description)

	if err := tmux.NewSession(sessionName, "coder", workDir, shellCmd); err != nil {
		if ccSessionPath != "" {
			os.Remove(ccSessionPath)
		}
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	if err := injectSessionEnv(sessionName, task, taskrc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	if cfg.Spawner != "" {
		if err := taskwarrior.SetSpawner(task.UUID, cfg.Spawner); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to set spawner: %v\n", err)
		}
	}

	fmt.Printf("\nWorker '%s' spawned successfully\n", cfg.Name)
	fmt.Printf("  Session: %s\n", sessionName)
	fmt.Printf("  Work dir: %s\n", workDir)
	fmt.Printf("\nTo attach:\n")
	fmt.Printf("  tmux attach -t %s\n", sessionName)

	return nil
}

// buildEnvParts returns the shared env vars for any runtime.
func buildEnvParts(task *taskwarrior.Task, rt runtime.Runtime, taskrc string) []string {
	parts := []string{
		"TTAL_AGENT_NAME=coder",
		fmt.Sprintf("TTAL_JOB_ID=%s", task.SessionID()),
		fmt.Sprintf("TTAL_RUNTIME=%s", rt),
	}
	if team := os.Getenv("TTAL_TEAM"); team != "" {
		parts = append(parts, fmt.Sprintf("TTAL_TEAM=%s", team))
	}
	if taskrc != "" {
		parts = append(parts, fmt.Sprintf("TASKRC=%s", taskrc))
	}

	return parts
}

func injectSessionEnv(sessionName string, task *taskwarrior.Task, taskrc string) error {
	setEnv := func(key, val string) {
		if err := tmux.SetEnv(sessionName, key, val); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to set %s: %v\n", key, err)
		}
	}

	setEnv("TTAL_JOB_ID", task.SessionID())

	team := os.Getenv("TTAL_TEAM")
	if team == "" {
		team = "default"
	}
	setEnv("TTAL_TEAM", team)

	if taskrc != "" {
		setEnv("TASKRC", taskrc)
	}

	// Inject allowlisted .env vars at session level (inherited by all windows).
	// Secrets (tokens) are blocked — authenticated operations go through the daemon.
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		return fmt.Errorf("failed to load .env for worker session: %w", err)
	}
	for k, v := range dotEnv {
		if !env.IsAllowedForSession(k) {
			continue // not on allowlist — daemon handles these
		}
		setEnv(k, v)
	}

	return nil
}

// writeSessionPrompt builds the system prompt for a CC worker session.
// This content goes into the synthetic JSONL session.
func writeSessionPrompt(task *taskwarrior.Task, cfg SpawnConfig, shellCfg *config.Config) (string, error) {
	shortID := task.UUID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	executePrompt := shellCfg.RenderPrompt("execute", shortID, cfg.Runtime)
	if executePrompt == "" {
		return "", fmt.Errorf("execute prompt not configured: add [prompts] execute = \"...\" to config.toml")
	}
	return executePrompt, nil
}

// writeTaskFile writes the system prompt to a temp file for Codex workers.
// Codex does not support JSONL resume (#321), so it uses the legacy task-file pattern.
func writeTaskFile(task *taskwarrior.Task, cfg SpawnConfig, shellCfg *config.Config) (string, error) {
	prompt, err := writeSessionPrompt(task, cfg, shellCfg)
	if err != nil {
		return "", err
	}

	taskFile, err := os.CreateTemp("", "claude-task-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create task file: %w", err)
	}
	if _, err := taskFile.WriteString(prompt); err != nil {
		_ = taskFile.Close()
		return "", fmt.Errorf("failed to write task file: %w", err)
	}
	_ = taskFile.Close()
	return taskFile.Name(), nil
}

func setupWorktree(project, dirName, branchName, projectAlias string) (string, error) {
	root := config.WorktreesRoot()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("failed to create worktree root %s: %w", root, err)
	}

	worktreeDir := filepath.Join(root, fmt.Sprintf("%s-%s", dirName, projectAlias))
	workerBranch := fmt.Sprintf("worker/%s", branchName)

	if info, err := os.Stat(worktreeDir); err == nil && info.IsDir() {
		fmt.Printf("Worktree already exists at %s, reusing\n", worktreeDir)
	} else {
		// Pull latest from remote so worktree branches from up-to-date main
		pullLatest(project)

		if err := createWorktree(project, worktreeDir, workerBranch); err != nil {
			return "", err
		}

		if err := pushBranchToUpstream(project, workerBranch); err != nil {
			fmt.Fprintf(os.Stderr, "  warning: failed to push branch (non-fatal): %v\n", err)
			fmt.Fprintf(os.Stderr, "  Worker can still function locally; push manually if needed.\n")
		} else {
			fmt.Printf("  Pushed branch to origin/%s\n", workerBranch)
		}
	}

	ensureWorktreeTrust(worktreeDir)
	return worktreeDir, nil
}

// ensureWorktreeTrust adds a trust entry for the worktree path in ~/.claude.json.
// Non-fatal: logs a warning on failure since the worker can still function
// (user just gets a manual trust prompt).
func ensureWorktreeTrust(worktreeDir string) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: cannot resolve home dir for trust entry: %v\n", err)
		return
	}
	claudeJSONPath := filepath.Join(home, ".claude.json")
	n, err := claudeconfig.UpsertTrust(claudeJSONPath, []string{worktreeDir})
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: failed to add trust entry (non-fatal): %v\n", err)
	} else if n > 0 {
		fmt.Printf("  Trust entry added for worktree: %s\n", worktreeDir)
	}
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

func pushBranchToUpstream(project, branch string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "-C", project, "remote", "get-url", "origin")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	// Check whether the remote branch already exists before pushing, so we can
	// skip the push without relying on locale-dependent git error messages.
	// (The old approach matched "branch is already" in stderr output, which
	// varies by git version and locale and is therefore fragile.)
	checkCmd := exec.CommandContext(ctx, "git", "-C", project, "ls-remote", "--exit-code", "--heads", "origin", branch)
	if checkCmd.Run() == nil {
		// Remote branch already exists; nothing to push.
		return nil
	}

	cmd = exec.CommandContext(ctx, "git", "-C", project, "push", "-u", "origin", branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to push branch %s: %w\n%s", branch, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// runWorktreeSetupWithFallback tries the target dir's .worktree-setup first,
// then falls back to the worktree root's script if target has none.
func runWorktreeSetupWithFallback(targetDir, worktreeRoot string) {
	setupScript := filepath.Join(targetDir, ".worktree-setup")
	if info, err := os.Stat(setupScript); err == nil && !info.IsDir() {
		runSetupScript(setupScript, targetDir)
		return
	}
	// Only fall back to root's script when targetDir is a subpath.
	// When targetDir == worktreeRoot, we already checked this dir above.
	if targetDir != worktreeRoot {
		runWorktreeSetup(worktreeRoot)
	}
}

func runWorktreeSetup(worktreeDir string) {
	setupScript := filepath.Join(worktreeDir, ".worktree-setup")
	info, err := os.Stat(setupScript)
	if err != nil || info.IsDir() {
		return
	}
	runSetupScript(setupScript, worktreeDir)
}

func runSetupScript(scriptPath, workDir string) {
	fmt.Printf("\nRunning %s...\n", filepath.Base(scriptPath))
	if err := os.Chmod(scriptPath, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "  warning: failed to make script executable: %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, scriptPath)
	cmd.Dir = workDir
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
	if b := gitutil.BranchName(workDir); b != "" {
		return b
	}
	return "unknown"
}

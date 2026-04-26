package worker

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/claudeconfig"
	"github.com/tta-lab/ttal-cli/internal/config"
	git "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// SpawnConfig holds configuration for spawning a worker.
type SpawnConfig struct {
	Name      string
	Project   string
	TaskUUID  string
	Worktree  bool
	Force     bool
	Runtime   runtime.Runtime
	AgentName string // CC agent identity from the pipeline stage assignee
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

	workDir, _, err := setupWorkDir(cfg, task, gitRoot)
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

	return launchTmuxWorker(cfg, task, sessionName, workDir)
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

// resolveRuntime determines the worker runtime from config, defaulting to ClaudeCode.
func resolveRuntime(rt runtime.Runtime, _ *taskwarrior.Task) runtime.Runtime {
	if rt != "" {
		return rt
	}
	shellCfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load config: %v\n", err)
	}
	if shellCfg != nil {
		return runtime.Runtime(shellCfg.DefaultRuntime)
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
		workDir, err = setupWorktree(project, task.HexID(), cfg.Name, task.Project)
		if err != nil {
			return "", "", fmt.Errorf("failed to setup worktree: %w", err)
		}
		return workDir, fmt.Sprintf("worker/%s", cfg.Name), nil
	}

	fmt.Println("Working in main directory (no worktree)")
	return project, detectBranch(project), nil
}

// buildRuntimeShellCommand builds the shell command for the given runtime.
func buildRuntimeShellCommand(
	cfg SpawnConfig, shellCfg *config.Config, ttalBin string, _ *taskwarrior.Task,
	agentName string, envParts []string,
) (string, error) {
	const trigger = "Run `ttal context` for your briefing, then act on the role prompt."
	switch cfg.Runtime {
	case runtime.Codex:
		taskFile, err := writeTaskFile()
		if err != nil {
			return "", err
		}
		codexCmd, err := launchcmd.BuildCodexGatekeeperCommand(ttalBin, taskFile)
		if err != nil {
			return "", err
		}
		return shellCfg.BuildEnvShellCommand(envParts, codexCmd), nil

	case runtime.Lenos:
		lenosCmd := launchcmd.BuildLenosCommand(ttalBin, agentName, trigger)
		return shellCfg.BuildEnvShellCommand(envParts, lenosCmd), nil

	default:
		ccCmd := launchcmd.BuildCCDirectCommand(ttalBin, agentName, trigger)
		return shellCfg.BuildEnvShellCommand(envParts, ccCmd), nil
	}
}

// launchTmuxWorker spawns a worker in a tmux session.
func launchTmuxWorker(
	cfg SpawnConfig, task *taskwarrior.Task, sessionName, workDir string,
) error {
	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	shellCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Use cfg.AgentName if set (always the case when spawned via pipeline advance).
	// Fall back to reading pipelines.toml to avoid hardcoding the agent name.
	agentName := cfg.AgentName
	if agentName == "" {
		if pipelineCfg, err := pipeline.Load(config.DefaultConfigDir()); err == nil {
			agentName = pipelineCfg.WorkerAgentName(task.Tags)
			if agentName == "" {
				agentName = pipelineCfg.AnyWorkerAgentName()
			}
		}
		if agentName == "" {
			agentName = CoderAgentName
		}
	}

	envParts := buildEnvParts(task, cfg.Runtime, agentName, shellCfg)

	shellCmd, err := buildRuntimeShellCommand(cfg, shellCfg, ttalBin, task, agentName, envParts)
	if err != nil {
		return err
	}

	fmt.Printf("\nLaunching %s with task: %s\n", cfg.Runtime, task.Description)

	if err := tmux.NewSession(sessionName, agentName, workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	fmt.Printf("\nWorker '%s' spawned successfully\n", cfg.Name)
	fmt.Printf("  Session: %s\n", sessionName)
	fmt.Printf("  Work dir: %s\n", workDir)
	fmt.Printf("\nTo attach:\n")
	fmt.Printf("  tmux attach -t %s\n", sessionName)

	return nil
}

// buildEnvParts returns the shared env vars for any runtime.
func buildEnvParts(task *taskwarrior.Task, rt runtime.Runtime, agentName string, _ *config.Config) []string {
	parts := []string{
		"TTAL_AGENT_NAME=" + agentName,
		fmt.Sprintf("TTAL_JOB_ID=%s", task.HexID()),
		fmt.Sprintf("TTAL_RUNTIME=%s", rt),
	}
	return parts
}

// writeTaskFile writes the unified spawn trigger to a temp file for Codex workers.
// Codex uses the task file as its initial prompt; the trigger tells Codex to run
// `ttal context` for its briefing.
func writeTaskFile() (string, error) {
	const trigger = "Run `ttal context` for your briefing, then act on the role prompt."

	taskFile, err := os.CreateTemp("", "codex-task-*.txt")
	if err != nil {
		return "", fmt.Errorf("failed to create task file: %w", err)
	}
	if _, err := taskFile.WriteString(trigger); err != nil {
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
		pullLatest(project, projectAlias)

		if err := createWorktree(project, worktreeDir, workerBranch); err != nil {
			return "", err
		}

		if err := pushBranchToUpstream(project, workerBranch, projectAlias); err != nil {
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

func pushBranchToUpstream(project, branch, projectAlias string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	remoteURL, err := gitutil.RemoteURL(project)
	if err != nil {
		return fmt.Errorf("failed to get origin remote: %w", err)
	}

	credEnv := gitutil.GitCredEnv(remoteURL, projectAlias)

	// Check whether the remote branch already exists before pushing, so we can
	// skip the push without relying on locale-dependent git error messages.
	// (The old approach matched "branch is already" in stderr output, which
	// varies by git version and locale and is therefore fragile.)
	checkCmd := exec.CommandContext(ctx, "git", "-C", project, "ls-remote", "--exit-code", "--heads", "origin", branch)
	checkCmd.Env = append(os.Environ(), credEnv...)
	if checkCmd.Run() == nil {
		// Remote branch already exists; nothing to push.
		return nil
	}

	cmd := exec.CommandContext(ctx, "git", "-C", project, "push", "-u", "origin", branch)
	cmd.Env = append(os.Environ(), credEnv...)
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

func pullLatest(project, projectAlias string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Println("Pulling latest changes...")
	cmd := exec.CommandContext(ctx, "git", "-C", project, "pull", "--ff-only")

	remoteURL, remoteErr := gitutil.RemoteURL(project)
	if remoteErr != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not get remote URL, pull runs without credentials: %v\n", remoteErr)
	} else {
		cmd.Env = append(os.Environ(), gitutil.GitCredEnv(remoteURL, projectAlias)...)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: git pull failed (non-fatal): %v\n", err)
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "  output: %s\n", strings.TrimSpace(string(out)))
		}
	}
}

func detectBranch(workDir string) string {
	if b := gitutil.BranchName(workDir); b != "" {
		return b
	}
	return "unknown"
}

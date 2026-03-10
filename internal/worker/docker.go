package worker

// Package-level doc: Docker workers run the coder window inside a Docker container
// while keeping the reviewer window on bare metal within the same tmux session.
// The container mounts the worktree, host socket, SSH keys, gitconfig, and a temp
// env file so the coder process gets all secrets without the daemon leaking them.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	gitutil "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// SpawnDocker is like Spawn but runs the coder window inside a Docker container.
// The reviewer window runs on bare metal. Both share the same tmux session.
func SpawnDocker(cfg SpawnConfig) error {
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

	gitRoot, err := gitutil.FindRoot(project)
	if err != nil {
		return fmt.Errorf("cannot find git root for %s: %w", project, err)
	}

	subpath := ""
	resolvedProject, _ := filepath.EvalSymlinks(project)
	resolvedRoot, _ := filepath.EvalSymlinks(gitRoot)
	if resolvedProject != resolvedRoot {
		rel, err := filepath.Rel(gitRoot, project)
		if err != nil {
			return fmt.Errorf("cannot compute relative subpath: %w", err)
		}
		subpath = rel
		fmt.Printf("  Monorepo subpath: %s\n", subpath)
	}

	cfg.Runtime = resolveRuntime(cfg.Runtime, task)
	if err := validateRuntime(cfg.Runtime); err != nil {
		return err
	}

	sessionName := task.SessionName()

	fmt.Printf("Spawning Docker %s worker: %s\n  Image: %s\n  Project: %s\n  Task: %s\n\n",
		cfg.Runtime, cfg.Name, cfg.Image, project, task.Description)
	fmt.Printf("Creating tmux session: %s\n", sessionName)

	if err := ensureSessionAvailable(cfg, sessionName, project); err != nil {
		return err
	}

	workDir, branch, err := setupWorkDir(cfg, gitRoot)
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

	if cfg.Worktree {
		runWorktreeSetupWithFallback(workDir, worktreeRoot)
	}

	return launchDockerTmuxWorker(cfg, task, sessionName, worktreeRoot, workDir, branch, project)
}

// launchDockerTmuxWorker spawns the coder window inside a Docker container.
// The reviewer window runs on bare metal (identical to bare-metal spawn).
func launchDockerTmuxWorker(
	cfg SpawnConfig,
	task *taskwarrior.Task,
	sessionName, worktreeRoot, workDir, branch, project string,
) error {
	ttalBin, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve ttal binary path: %w", err)
	}

	shellCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	taskFile, err := writeTaskFile(task, cfg, shellCfg)
	if err != nil {
		return err
	}

	taskrc := resolveTaskRCFromConfig(shellCfg)
	envParts := buildEnvParts(task, cfg.Runtime, taskrc)
	model := resolveModel(task, shellCfg)

	// Build the bare-metal launch command (gatekeeper + runtime args).
	runtimeCmd, err := launchcmd.BuildGatekeeperCommand(ttalBin, taskFile, cfg.Runtime, model)
	if err != nil {
		return err
	}

	// Write a temp env file for Docker (--env-file). This delivers secrets to the
	// container without embedding them in the docker run command string.
	envFile, err := writeDockerEnvFile(taskrc)
	if err != nil {
		return fmt.Errorf("failed to write docker env file: %w", err)
	}

	// Resolve host socket path for mounting into the container.
	hostSock, err := hostSocketPath()
	if err != nil {
		return fmt.Errorf("failed to resolve socket path: %w", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home dir: %w", err)
	}
	sshDir := filepath.Join(home, ".ssh")
	gitconfigPath := filepath.Join(home, ".gitconfig")

	// workDir inside the container mirrors the host path (same absolute path).
	dockerCmd := buildDockerCmd(
		task.SessionID(),
		string(cfg.Runtime),
		cfg.Image,
		os.Getuid(),
		os.Getgid(),
		worktreeRoot,
		taskFile,
		hostSock,
		sshDir,
		gitconfigPath,
		envFile,
		runtimeCmd,
	)

	// Wrap with env vars so they land in the tmux session (picked up by reviewer window).
	shellCmd := shellCfg.BuildEnvShellCommand(envParts, dockerCmd)

	fmt.Printf("\nLaunching Docker %s with task: %s\n", cfg.Runtime, task.Description)

	if err := tmux.NewSession(sessionName, "worker", workDir, shellCmd); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Reviewer window runs on bare metal and inherits session-level env.
	if err := injectSessionEnv(sessionName, task, taskrc); err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	if cfg.Spawner != "" {
		if err := taskwarrior.SetSpawner(task.UUID, cfg.Spawner); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to set spawner: %v\n", err)
		}
	}

	if err := taskwarrior.UpdateWorkerMetadata(task.UUID, branch, project); err != nil {
		return fmt.Errorf("session created but task tracking failed\n"+
			"  Session: %s\n"+
			"  To attach: tmux attach -t %s\n\n"+
			"  %w", sessionName, sessionName, err)
	}

	fmt.Printf("\nDocker worker '%s' spawned successfully\n", cfg.Name)
	fmt.Printf("  Session: %s\n  Work dir: %s\n  Image: %s\n", sessionName, workDir, cfg.Image)
	fmt.Printf("\nTo attach:\n  tmux attach -t %s\n", sessionName)

	return nil
}

// buildDockerCmd constructs the docker run command string.
// The worktree, task file, host socket, SSH keys, gitconfig and env file are all
// mounted read-only (except the worktree itself which needs write access for git).
func buildDockerCmd(
	sessionID, runtime, image string,
	uid, gid int,
	worktreeRoot, taskFile, hostSock, sshDir, gitconfigPath, envFile, runtimeCmd string,
) string {
	parts := []string{
		"docker", "run", "--rm", "-it",
		fmt.Sprintf("--name=ttal-worker-%s", sessionID),
		fmt.Sprintf("--user=%d:%d", uid, gid),
		// Worktree as /workspace (read-write for git operations)
		fmt.Sprintf("--volume=%s:/workspace", worktreeRoot),
		// Task file (read-only)
		fmt.Sprintf("--volume=%s:%s:ro", taskFile, taskFile),
		// Daemon socket (read-write so ttal send works)
		fmt.Sprintf("--volume=%s:/run/ttal.sock", hostSock),
		// SSH keys (read-only)
		fmt.Sprintf("--volume=%s:/home/agent/.ssh:ro", sshDir),
		// gitconfig (read-only)
		fmt.Sprintf("--volume=%s:/home/agent/.gitconfig:ro", gitconfigPath),
		// Secrets env file
		fmt.Sprintf("--env-file=%s", envFile),
		// Point ttal at the mounted socket path inside the container
		"--env=TTAL_SOCKET_PATH=/run/ttal.sock",
		"--env=TTAL_ROLE=coder",
		fmt.Sprintf("--env=TTAL_JOB_ID=%s", sessionID),
		fmt.Sprintf("--env=TTAL_RUNTIME=%s", runtime),
		image,
		// The runtime command (gatekeeper + CC/OpenCode) becomes the container CMD.
		// Wrap in bash -c so shell features (&&, env substitution) work correctly.
		"bash", "-c", runtimeCmd,
	}
	return strings.Join(parts, " ")
}

// writeDockerEnvFile writes a temp env file with all secrets from ~/.config/ttal/.env.
// Docker --env-file format: KEY=VALUE, one per line.
func writeDockerEnvFile(taskrc string) (string, error) {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		// Non-fatal — worker launches without API secrets
		fmt.Fprintf(os.Stderr, "warning: docker worker env file: %v\n", err)
		dotEnv = map[string]string{}
	}

	f, err := os.CreateTemp("", "ttal-docker-env-*.env")
	if err != nil {
		return "", fmt.Errorf("creating env file: %w", err)
	}
	defer f.Close()

	// Restrict to owner-only — contains API secrets
	if err := os.Chmod(f.Name(), 0o600); err != nil {
		return "", fmt.Errorf("chmod env file: %w", err)
	}

	for k, v := range dotEnv {
		fmt.Fprintf(f, "%s=%s\n", k, v)
	}
	if taskrc != "" {
		fmt.Fprintf(f, "TASKRC=%s\n", taskrc)
	}
	if team := os.Getenv("TTAL_TEAM"); team != "" {
		fmt.Fprintf(f, "TTAL_TEAM=%s\n", team)
	}

	return f.Name(), nil
}

// hostSocketPath returns the daemon socket path on the host.
// Ignores TTAL_SOCKET_PATH (which is for containers) and always returns the
// host default, so we know the correct path to mount into the container.
func hostSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "daemon.sock"), nil
}

package worker

// Docker workers run the coder window inside a Docker container while keeping
// the reviewer window on bare metal within the same tmux session.
// The container mounts the worktree, host socket, SSH keys, gitconfig, and an
// env file (co-located in the worktree so it's cleaned up with the worktree).

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

	subpath, err := resolveSubpath(project, gitRoot)
	if err != nil {
		return err
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
	model := resolveModel(task, shellCfg)

	// Build the runtime command (gatekeeper + CC/Codex invocation).
	runtimeCmd, err := launchcmd.BuildGatekeeperCommand(ttalBin, taskFile, cfg.Runtime, model, "")
	if err != nil {
		return err
	}

	// Write env file co-located in the worktree so it's cleaned up automatically
	// when the worktree is removed by worker.Close.
	envFile, err := writeDockerEnvFile(taskrc, worktreeRoot)
	if err != nil {
		return fmt.Errorf("failed to write docker env file: %w", err)
	}

	// Resolve host socket path for mounting into the container.
	// hostSocketPath() always returns the host default (ignores TTAL_SOCKET_PATH).
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

	// buildDockerShellCmd produces a properly shell-quoted command string that
	// tmux passes to the shell. Using shellQuote on all host paths and the
	// runtimeCmd prevents breakage when paths contain spaces.
	dockerShellCmd := buildDockerShellCmd(
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

	fmt.Printf("\nLaunching Docker %s with task: %s\n", cfg.Runtime, task.Description)

	// Pass dockerShellCmd directly — env vars for the reviewer window are set by
	// injectSessionEnv below. The Docker container gets its env via --env-file and --env=.
	if err := tmux.NewSession(sessionName, "worker", workDir, dockerShellCmd); err != nil {
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

// buildDockerShellCmd constructs a shell-safe docker run command string.
// All host paths are shell-quoted so the command is safe when passed through
// the user's shell (e.g. `tmux new-session ... 'docker run ...'`).
func buildDockerShellCmd(
	sessionID, rt, image string,
	uid, gid int,
	worktreeRoot, taskFile, hostSock, sshDir, gitconfigPath, envFile, runtimeCmd string,
) string {
	sq := shellQuoteStr
	parts := []string{
		"docker", "run", "--rm", "-it",
		"--name=ttal-worker-" + sessionID,
		fmt.Sprintf("--user=%d:%d", uid, gid),
		// --workdir makes the working directory explicit (base image sets WORKDIR /workspace
		// but worker images could override it).
		"--workdir=/workspace",
		// Worktree as /workspace (read-write for git operations)
		"--volume=" + sq(worktreeRoot) + ":/workspace",
		// Task file (read-only; same path inside container as on host)
		"--volume=" + sq(taskFile) + ":" + sq(taskFile) + ":ro",
		// Daemon socket (read-write so ttal send works from inside the container)
		"--volume=" + sq(hostSock) + ":/run/ttal.sock",
		// SSH keys (read-only)
		"--volume=" + sq(sshDir) + ":/home/agent/.ssh:ro",
		// gitconfig (read-only)
		"--volume=" + sq(gitconfigPath) + ":/home/agent/.gitconfig:ro",
		// Secrets env file (co-located in worktree, cleaned up with worktree)
		"--env-file=" + sq(envFile),
		// Point ttal at the mounted socket path inside the container
		"--env=TTAL_SOCKET_PATH=/run/ttal.sock",
		"--env=TTAL_ROLE=coder",
		"--env=TTAL_JOB_ID=" + sessionID,
		"--env=TTAL_RUNTIME=" + rt,
		sq(image),
		// bash -c <quoted-runtimeCmd>: shell-quoting runtimeCmd ensures it is
		// treated as a single argument even when it contains spaces.
		"bash", "-c", sq(runtimeCmd),
	}
	return strings.Join(parts, " ")
}

// writeDockerEnvFile writes an env file with secrets from ~/.config/ttal/.env.
// The file is placed inside worktreeRoot so it is automatically deleted when the
// worktree is cleaned up by worker.Close, avoiding persistent secret leakage.
// Docker --env-file format: KEY=VALUE, one per line.
func writeDockerEnvFile(taskrc, worktreeRoot string) (string, error) {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		// Non-fatal — worker launches without API secrets
		fmt.Fprintf(os.Stderr, "warning: docker worker env file: %v\n", err)
		dotEnv = map[string]string{}
	}

	envPath := filepath.Join(worktreeRoot, ".ttal-docker-env")
	f, err := os.OpenFile(envPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("creating env file: %w", err)
	}
	defer f.Close()

	for k, v := range dotEnv {
		fmt.Fprintf(f, "%s=%s\n", k, v)
	}
	if taskrc != "" {
		fmt.Fprintf(f, "TASKRC=%s\n", taskrc)
	}
	if team := os.Getenv("TTAL_TEAM"); team != "" {
		fmt.Fprintf(f, "TTAL_TEAM=%s\n", team)
	}

	return envPath, nil
}

// hostSocketPath returns the daemon socket path on the host.
// Ignores TTAL_SOCKET_PATH (which is for containers pointing back at the host socket)
// so we always get the real host path to mount into the container.
func hostSocketPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".ttal", "daemon.sock"), nil
}

// resolveSubpath computes the relative subpath from gitRoot to project.
// Returns ("", nil) when they are the same directory (not a monorepo subpath).
// Logs a warning and falls back to unresolved paths when EvalSymlinks fails.
func resolveSubpath(project, gitRoot string) (string, error) {
	resolvedProject, err := filepath.EvalSymlinks(project)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: EvalSymlinks(%s): %v — using unresolved path\n", project, err)
		resolvedProject = project
	}
	resolvedRoot, err := filepath.EvalSymlinks(gitRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: EvalSymlinks(%s): %v — using unresolved path\n", gitRoot, err)
		resolvedRoot = gitRoot
	}

	if resolvedProject == resolvedRoot {
		return "", nil
	}

	rel, err := filepath.Rel(gitRoot, project)
	if err != nil {
		return "", fmt.Errorf("cannot compute relative subpath: %w", err)
	}
	fmt.Printf("  Monorepo subpath: %s\n", rel)
	return rel, nil
}

// shellQuoteStr wraps s in single quotes for POSIX shell, escaping embedded single quotes.
// Identical to daemon/k8s.go shellQuote — kept local to avoid cross-package dependency.
func shellQuoteStr(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

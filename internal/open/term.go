package open

import (
	"fmt"
	"os"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// Term opens a terminal (shell) in the task's working directory (worktree or project root).
func Term(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		return err
	}

	if task.ProjectPath == "" {
		return fmt.Errorf("no project path associated with this task: missing project_path UDA")
	}

	workDir := resolveWorkDir(task)

	if _, err := os.Stat(workDir); err != nil {
		return fmt.Errorf("directory not found: %s", workDir)
	}

	shell := resolveShell()

	shellBin, err := lookPath(shell)
	if err != nil {
		return fmt.Errorf("shell %q not found in PATH", shell)
	}

	fmt.Printf("Opening terminal in %s...\n", workDir)

	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to chdir to %s: %w", workDir, err)
	}

	return syscall.Exec(shellBin, []string{shell}, os.Environ())
}

func resolveShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

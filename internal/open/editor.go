package open

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func ttalWorktreeRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".ttal-worktrees"
	}
	return filepath.Join(home, ".ttal", "worktrees")
}

// Editor opens the task's project directory (or worktree) in an editor.
func Editor(uuid string) error {
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

	editor := resolveEditor()

	editorBin, err := lookPath(editor)
	if err != nil {
		return fmt.Errorf("editor %q not found in PATH", editor)
	}

	fmt.Printf("Opening %s in %s...\n", workDir, editor)

	if err := os.Chdir(workDir); err != nil {
		return fmt.Errorf("failed to chdir to %s: %w", workDir, err)
	}

	return syscall.Exec(editorBin, []string{editor, "."}, os.Environ())
}

func resolveWorkDir(task *taskwarrior.Task) string {
	// Try worktree by branch name (without worker/ prefix)
	if task.Branch != "" {
		name := strings.TrimPrefix(task.Branch, "worker/")
		dir := filepath.Join(ttalWorktreeRoot(), name)
		if isDir(dir) {
			return dir
		}
	}

	return task.ProjectPath
}

func resolveEditor() string {
	if e := os.Getenv("TT_EDITOR"); e != "" {
		return e
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

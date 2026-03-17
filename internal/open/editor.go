package open

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/flicktask"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// Editor opens the task's project directory (or worktree) in an editor.
func Editor(uuid string) error {
	if err := flicktask.ValidateID(uuid); err != nil {
		return err
	}

	task, err := flicktask.ExportTask(uuid)
	if err != nil {
		return err
	}

	projectPath, err := project.ResolveProjectPathOrError(task.Project)
	if err != nil {
		return err
	}

	workDir := resolveWorkDir(task, projectPath)

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

func resolveWorkDir(task *flicktask.Task, projectPath string) string {
	if task.UUID != "" && task.Project != "" {
		worktreeRoot := config.EnsureWorktreeRoot()
		dir := filepath.Join(worktreeRoot, fmt.Sprintf("%s-%s", task.UUID[:8], task.Project))
		if isDir(dir) {
			return dir
		}
	}

	return projectPath
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

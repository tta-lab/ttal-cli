package worker

import (
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// TaskHexFromCwd extracts the hex task ID from a worktree CWD path.
// Worktree dirs follow the pattern ~/.ttal/worktrees/<hexID>-<alias>.
// Returns "" if the CWD is not under a recognised worktree directory.
func TaskHexFromCwd(cwd string) string {
	if cwd == "" {
		return ""
	}
	worktreesRoot := config.WorktreesRoot()
	if !strings.HasPrefix(cwd, worktreesRoot+string(filepath.Separator)) {
		return ""
	}
	rel := strings.TrimPrefix(cwd, worktreesRoot+string(filepath.Separator))
	dirName := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	parts := strings.SplitN(dirName, "-", 2)
	return parts[0]
}

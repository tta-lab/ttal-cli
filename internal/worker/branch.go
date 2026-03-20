package worker

import (
	"fmt"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
)

// WorktreeBranch returns the current git branch for a task's worktree.
// Derives worktree path from task UUID + project alias (no branch UDA needed).
// Returns error if worktree doesn't exist or has no branch.
func WorktreeBranch(taskUUID, projectAlias string) (string, error) {
	if len(taskUUID) < 8 {
		return "", fmt.Errorf("task UUID too short: %s", taskUUID)
	}
	worktreeDir := filepath.Join(config.WorktreesRoot(),
		fmt.Sprintf("%s-%s", taskUUID[:8], projectAlias))
	branch := gitutil.BranchName(worktreeDir)
	if branch == "" {
		return "", fmt.Errorf("no branch found in worktree %s", worktreeDir)
	}
	return branch, nil
}

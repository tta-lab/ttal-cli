package worker

import (
	"fmt"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
)

// WorktreePath returns the filesystem path of the worktree for the given task UUID
// and project alias. Returns an error if the UUID is too short (< 8 chars).
func WorktreePath(taskUUID, projectAlias string) (string, error) {
	if len(taskUUID) < 8 {
		return "", fmt.Errorf("task UUID too short: %s", taskUUID)
	}
	return filepath.Join(config.WorktreesRoot(),
		fmt.Sprintf("%s-%s", taskUUID[:8], projectAlias)), nil
}

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

package worker

import (
	"fmt"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/project"
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

// CurrentBranch returns the current git branch for a task, supporting both
// worktree and non-worktree (regular git repo) setups.
//
// Resolution order:
//  1. If worktree exists at ~/.ttal/worktrees/<uuid8>-<alias>/ → use it
//  2. If project path exists → run git branch --show-current there
//  3. If workDir is provided and is a valid git repo → run git branch --show-current there
//
// Returns empty string if no branch can be determined.
func CurrentBranch(taskUUID, projectAlias, workDir string) string {
	// Try worktree first
	if len(taskUUID) >= 8 {
		worktreeDir := filepath.Join(config.WorktreesRoot(),
			fmt.Sprintf("%s-%s", taskUUID[:8], projectAlias))
		if branch := gitutil.BranchName(worktreeDir); branch != "" {
			return branch
		}
	}

	// Try project path
	if projectAlias != "" {
		proj := resolveProjectPath(projectAlias)
		if proj != "" {
			if branch := gitutil.BranchName(proj); branch != "" {
				return branch
			}
		}
	}

	// Try provided workDir
	if workDir != "" {
		return gitutil.BranchName(workDir)
	}

	return ""
}

// resolveProjectPath resolves a project alias to its filesystem path.
// Returns empty string if not found.
func resolveProjectPath(alias string) string {
	store := project.NewStore(config.ResolveProjectsPath())
	proj, err := store.Get(alias)
	if err != nil || proj == nil {
		return ""
	}
	return proj.Path
}

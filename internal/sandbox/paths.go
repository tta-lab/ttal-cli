package sandbox

import (
	"log"
	"path/filepath"
	"sort"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// CollectProjectGitDirs returns deduplicated .git directories for all registered projects.
// Used by both ttal sync (CC-session sandbox) and ttal subagent run (temenos AllowedPaths)
// to ensure git commands work in worktrees whose .git file points back to the main repo.
func CollectProjectGitDirs() []string {
	storePath := config.ResolveProjectsPath()
	store := project.NewStore(storePath)
	projects, err := store.List(false)
	if err != nil {
		log.Printf("[sandbox] warning: failed to load projects for git dirs: %v"+
			" — git write access will be missing", err)
		return nil
	}

	seen := make(map[string]bool)
	var gitDirs []string
	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		gitDir := resolveGitDir(p.Path)
		if gitDir != "" && !seen[gitDir] {
			seen[gitDir] = true
			gitDirs = append(gitDirs, gitDir)
		}
	}
	sort.Strings(gitDirs)
	return gitDirs
}

// resolveGitDir returns the .git directory for a project path.
// For linked worktrees, returns the common git dir. For regular repos, returns <path>/.git.
func resolveGitDir(projectPath string) string {
	if commonDir := gitutil.LinkedWorktreeCommonDir(projectPath); commonDir != "" {
		return commonDir
	}
	return filepath.Join(projectPath, ".git")
}

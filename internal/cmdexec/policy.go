package cmdexec

import (
	"os"
	"path/filepath"

	"github.com/tta-lab/logos"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// DefaultTimeoutSec is the default per-command timeout.
const DefaultTimeoutSec = 120

// PolicyForAgent returns the sandbox policy for a manager agent.
//
// Read-only: every active project path from projects.toml.
// Read-write: the agent's own workspace (cwd).
// Returns (nil, false) if agentCwd is empty or the store cannot be read.
func PolicyForAgent(projectStore *project.Store, agentCwd string) ([]logos.AllowedPath, bool) {
	if agentCwd == "" {
		return nil, false
	}

	absCwd, err := filepath.Abs(agentCwd)
	if err != nil {
		return nil, false
	}

	var paths []logos.AllowedPath

	// Agent's own workspace: read-write.
	paths = append(paths, logos.AllowedPath{Path: absCwd, ReadOnly: false})

	// All active projects: read-only.
	projects, err := projectStore.List(false)
	if err != nil {
		// Fall back to agent workspace only if we can't read projects.
		return paths, true
	}

	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		absPath, err := filepath.Abs(p.Path)
		if err != nil {
			continue
		}
		// Skip the agent's own cwd (already added as rw).
		if absPath == absCwd {
			continue
		}
		paths = append(paths, logos.AllowedPath{Path: absPath, ReadOnly: true})
	}

	return paths, true
}

// PolicyForAgentFS is like PolicyForAgent but reads projects from the filesystem
// path directly (useful when a Store is not yet constructed).
func PolicyForAgentFS(projectsPath, agentCwd string) ([]logos.AllowedPath, bool) {
	if agentCwd == "" {
		return nil, false
	}

	absCwd, err := filepath.Abs(agentCwd)
	if err != nil {
		return nil, false
	}

	var paths []logos.AllowedPath

	// Agent's own workspace: read-write.
	paths = append(paths, logos.AllowedPath{Path: absCwd, ReadOnly: false})

	// Try to read projects.toml directly.
	store := project.NewStore(filepath.Join(projectsPath, "projects.toml"))
	projects, err := store.List(false)
	if err != nil || len(projects) == 0 {
		return paths, true
	}

	for _, p := range projects {
		if p.Path == "" {
			continue
		}
		absPath, err := filepath.Abs(p.Path)
		if err != nil {
			continue
		}
		if absPath == absCwd {
			continue
		}
		paths = append(paths, logos.AllowedPath{Path: absPath, ReadOnly: true})
	}

	return paths, true
}

// expandHome expands ~ in a path to the user's home directory.
func expandHome(path string) string {
	if len(path) < 2 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}

package env

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sharedTemenosPaths returns the base allowed paths shared by all session types.
// All use :rw because SQLite WAL mode requires write access even for reads,
// except ~/.config/ttal which is config-only.
func sharedTemenosPaths() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for temenos paths: %w", err)
	}
	return []string{
		filepath.Join(home, ".ttal") + ":rw",
		filepath.Join(home, ".task") + ":rw",
		filepath.Join(home, ".diary") + ":rw",
		filepath.Join(home, ".local", "share", "flicknote") + ":rw",
		filepath.Join(home, ".config", "ttal") + ":ro",
	}, nil
}

// buildTemenosEnv assembles TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts.
// extraPaths are appended as :ro entries after the shared paths.
func buildTemenosEnv(write bool, extraPaths []string) ([]string, error) {
	shared, err := sharedTemenosPaths()
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(shared)+len(extraPaths))
	paths = append(paths, shared...)
	for _, p := range extraPaths {
		paths = append(paths, p+":ro")
	}
	writeStr := "false"
	if write {
		writeStr = "true"
	}
	return []string{
		"TEMENOS_WRITE=" + writeStr,
		fmt.Sprintf("TEMENOS_PATHS=%s", strings.Join(paths, ",")),
		"ENABLE_TOOL_SEARCH=false",
	}, nil
}

// WorkerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for worker sessions.
// Workers get write access to cwd (worktree) via TEMENOS_WRITE=true.
func WorkerTemenosEnv() ([]string, error) {
	return buildTemenosEnv(true, nil)
}

// ReviewerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for reviewer sessions.
// Reviewers get read-only access to cwd (worktree) via TEMENOS_WRITE=false.
func ReviewerTemenosEnv() ([]string, error) {
	return buildTemenosEnv(false, nil)
}

// ManagerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for manager sessions.
// Managers get read-only cwd (TEMENOS_WRITE=false) plus all project paths as :ro
// for code investigation.
func ManagerTemenosEnv(projectPaths []string) ([]string, error) {
	return buildTemenosEnv(false, projectPaths)
}

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

// WorkerTemenosEnv returns TEMENOS_WRITE and TEMENOS_PATHS env parts for worker sessions.
// Workers get write access to cwd (worktree) via TEMENOS_WRITE=true.
func WorkerTemenosEnv() []string {
	paths, err := sharedTemenosPaths()
	if err != nil {
		return []string{"TEMENOS_WRITE=true"}
	}
	return []string{
		"TEMENOS_WRITE=true",
		fmt.Sprintf("TEMENOS_PATHS=%s", strings.Join(paths, ",")),
	}
}

// ReviewerTemenosEnv returns TEMENOS_WRITE and TEMENOS_PATHS env parts for reviewer sessions.
// Reviewers get read-only access to cwd (worktree) via TEMENOS_WRITE=false.
func ReviewerTemenosEnv() []string {
	paths, err := sharedTemenosPaths()
	if err != nil {
		return []string{"TEMENOS_WRITE=false"}
	}
	return []string{
		"TEMENOS_WRITE=false",
		fmt.Sprintf("TEMENOS_PATHS=%s", strings.Join(paths, ",")),
	}
}

// ManagerTemenosEnv returns TEMENOS_WRITE and TEMENOS_PATHS env parts for manager sessions.
// Managers get read-only cwd (TEMENOS_WRITE=false) plus all project paths as :ro
// for code investigation.
func ManagerTemenosEnv(projectPaths []string) []string {
	paths, err := sharedTemenosPaths()
	if err != nil {
		return []string{"TEMENOS_WRITE=false"}
	}
	for _, p := range projectPaths {
		paths = append(paths, p+":ro")
	}
	return []string{
		"TEMENOS_WRITE=false",
		fmt.Sprintf("TEMENOS_PATHS=%s", strings.Join(paths, ",")),
	}
}

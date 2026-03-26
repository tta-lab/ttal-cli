package env

import (
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// buildTemenosEnv assembles TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts.
// plane is "worker" or "manager" — selects which sandbox.toml section to merge.
// extraPaths are appended as :ro entries after sandbox paths.
func buildTemenosEnv(write bool, plane string, extraPaths []string) ([]string, error) {
	sandbox := config.LoadSandbox().PathsForPlane(plane)
	paths := make([]string, 0, len(sandbox)+len(extraPaths))
	paths = append(paths, sandbox...)
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
// Workers get write access to cwd (worktree) via TEMENOS_WRITE=true,
// plus read-only access to extraReadOnlyPaths (project paths, references).
func WorkerTemenosEnv(extraReadOnlyPaths []string) ([]string, error) {
	return buildTemenosEnv(true, "worker", extraReadOnlyPaths)
}

// ReviewerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for reviewer sessions.
// Reviewers get read-only cwd via TEMENOS_WRITE=false,
// plus read-only access to extraReadOnlyPaths (project paths, references).
func ReviewerTemenosEnv(extraReadOnlyPaths []string) ([]string, error) {
	return buildTemenosEnv(false, "worker", extraReadOnlyPaths)
}

// ManagerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for manager sessions.
// Managers get read-only cwd (TEMENOS_WRITE=false) plus all project paths as :ro
// for code investigation.
func ManagerTemenosEnv(projectPaths []string) ([]string, error) {
	return buildTemenosEnv(false, "manager", projectPaths)
}

// CollectReadOnlyPaths returns all registered project paths plus the ask
// references_path for use as read-only TEMENOS_PATHS entries.
// Loads project store and config. Non-fatal on errors — returns what it can.
// Only includes references_path if the directory actually exists on disk
// (AskReferencesPath always returns a default even on fresh installs).
func CollectReadOnlyPaths() []string {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[env] warning: failed to load config for read-only paths: %v", err)
		return nil
	}

	storePath := config.ResolveProjectsPath()
	store := project.NewStore(storePath)
	projects, err := store.List(false)
	if err != nil {
		log.Printf("[env] warning: failed to load projects for TEMENOS_PATHS: %v", err)
	}

	seen := make(map[string]bool)
	var paths []string
	for _, p := range projects {
		if p.Path != "" && !seen[p.Path] {
			seen[p.Path] = true
			paths = append(paths, p.Path)
		}
	}

	refsPath := cfg.AskReferencesPath()
	if refsPath != "" && !seen[refsPath] {
		if _, statErr := os.Stat(refsPath); statErr == nil {
			paths = append(paths, refsPath)
		}
	}

	sort.Strings(paths)
	return paths
}

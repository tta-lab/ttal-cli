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
func buildTemenosEnv(write bool, plane string, extraPaths []string) []string {
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
	}
}

// WorkerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for worker sessions.
// Workers get write access to cwd (worktree) via TEMENOS_WRITE=true,
// plus read-only access to extraReadOnlyPaths (project paths, references).
func WorkerTemenosEnv(extraReadOnlyPaths []string) []string {
	return buildTemenosEnv(true, "worker", extraReadOnlyPaths)
}

// ReviewerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for reviewer sessions.
// Reviewers get read-only cwd via TEMENOS_WRITE=false,
// plus read-only access to extraReadOnlyPaths (project paths, references).
func ReviewerTemenosEnv(extraReadOnlyPaths []string) []string {
	return buildTemenosEnv(false, "worker", extraReadOnlyPaths)
}

// ManagerTemenosEnv returns TEMENOS_WRITE, TEMENOS_PATHS, and ENABLE_TOOL_SEARCH env parts for manager sessions.
// Managers get read-only cwd (TEMENOS_WRITE=false) plus all project paths as :ro
// for code investigation.
func ManagerTemenosEnv(projectPaths []string) []string {
	return buildTemenosEnv(false, "manager", projectPaths)
}

// AppendTemenosPath appends a path entry (with :ro or :rw suffix) to the
// TEMENOS_PATHS value in a temenos env slice. The entry is appended as-is
// (caller must include the suffix). Returns the modified slice.
func AppendTemenosPath(temenosEnv []string, entry string) []string {
	const prefix = "TEMENOS_PATHS="
	for i, v := range temenosEnv {
		if strings.HasPrefix(v, prefix) {
			existing := strings.TrimPrefix(v, prefix)
			if existing == "" {
				temenosEnv[i] = prefix + entry
			} else {
				temenosEnv[i] = v + "," + entry
			}
			return temenosEnv
		}
	}
	return append(temenosEnv, prefix+entry)
}

// CollectReadOnlyPaths returns all registered project paths plus the ask
// references_path for use as read-only TEMENOS_PATHS entries.
// Loads project store and config. Non-fatal on errors — returns what it can.
//
// refsPath is added here (not in sandbox.toml) because it is user-configurable
// at runtime and may point to an arbitrary directory outside any statically-known
// sandbox path. The static sandbox.toml covers well-known tool dirs; dynamic
// per-user paths are injected at spawn time via this function.
//
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

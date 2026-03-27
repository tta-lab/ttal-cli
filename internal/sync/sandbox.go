package sync

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// SandboxResult holds the outcome of a SyncSandbox call.
type SandboxResult struct {
	AllowWritePaths []string
	DenyReadPaths   []string
	GitDirCount     int
}

// SyncSandbox updates ~/.claude/settings.json with sandbox and secret-deny config.
// It reads the project store for paths and .git dirs, loads sandbox.toml for
// allowWrite/denyRead/allowRead/network config, and merges into the existing settings.json.
func SyncSandbox(dryRun bool) (SandboxResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SandboxResult{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	return syncSandbox(dryRun, settingsPath)
}

func syncSandbox(dryRun bool, settingsPath string) (SandboxResult, error) {
	sandbox, err := config.LoadSandboxWithError()
	if err != nil {
		return SandboxResult{}, fmt.Errorf("loading sandbox.toml: %w", err)
	}
	if !sandbox.Enabled {
		return SandboxResult{}, nil
	}

	allowWrite, gitDirCount := buildAllowWritePaths(sandbox)
	denyRead := buildDenyReadPaths(sandbox)

	result := SandboxResult{
		AllowWritePaths: allowWrite,
		DenyReadPaths:   denyRead,
		GitDirCount:     gitDirCount,
	}

	if dryRun {
		return result, nil
	}

	settings, err := readOrInitSettings(settingsPath)
	if err != nil {
		return result, err
	}

	// Replace sandbox section, preserving any existing user unix sockets.
	existingSockets := extractExistingSockets(settings)
	allowRead := sandbox.ExpandedAllowRead()
	denyWrite := sandbox.ExpandedDenyWrite()
	settings["sandbox"] = buildSandboxSection(allowWrite, denyWrite, denyRead, allowRead, sandbox.Network.AllowedDomains, sandbox.AutoAllowBashIfSandboxed, existingSockets)

	// Append permissions.deny entries from sandbox.toml (additive, preserve existing).
	perms, denySlice, err := extractPermsDenyList(settings)
	if err != nil {
		return result, err
	}
	denySlice = appendPermsDenyEntries(denySlice, sandbox.ExpandedPermissionsDeny())
	if denySlice == nil {
		denySlice = []interface{}{}
	}
	perms["deny"] = denySlice
	settings["permissions"] = perms

	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		return result, fmt.Errorf("writing settings.json: %w", err)
	}

	return result, nil
}

// buildAllowWritePaths collects all paths that should be in allowWrite:
// - allowWrite entries from sandbox.toml (raw — no existence filtering)
// - .git dirs for all registered projects (deduplicated)
func buildAllowWritePaths(sandbox *config.SandboxConfig) ([]string, int) {
	seen := make(map[string]bool)
	var paths []string

	// sandbox.toml allowWrite paths (no existence filtering — declarative config)
	for _, p := range sandbox.AllowWrite {
		expanded := expandHomePath(p)
		if !seen[expanded] {
			seen[expanded] = true
			paths = append(paths, expanded)
		}
	}

	// Project .git dirs
	gitDirCount := 0
	for _, gitDir := range collectProjectGitDirs() {
		if !seen[gitDir] {
			seen[gitDir] = true
			paths = append(paths, gitDir)
			gitDirCount++
		}
	}

	sort.Strings(paths)
	return paths, gitDirCount
}

// buildDenyReadPaths returns the expanded list of paths to deny reads for,
// sourced from sandbox.toml's denyRead field.
func buildDenyReadPaths(sandbox *config.SandboxConfig) []string {
	return sandbox.ExpandedDenyRead()
}

// daemonSocketPath is the unix socket used for agent↔daemon communication.
// Always whitelisted so that ttal send/go work in all agent sessions.
const daemonSocketPath = "~/.ttal/daemon.sock"

// buildSandboxSection constructs the full sandbox object for settings.json.
// failIfUnavailable and allowUnsandboxedCommands are hardcoded secure defaults.
// autoAllowBashIfSandboxed is written only when explicitly set (non-nil).
// allowRead, denyWrite, allowedDomains are omitted when empty.
// existingSockets are user-defined unix sockets from a prior settings.json; they
// are preserved and our daemonSocketPath is appended (deduplicated).
func buildSandboxSection(allowWrite, denyWrite, denyRead, allowRead, allowedDomains []string, autoAllowBash *bool, existingSockets []string) map[string]interface{} {
	toIfaceSlice := func(ss []string) []interface{} {
		out := make([]interface{}, len(ss))
		for i, s := range ss {
			out[i] = s
		}
		return out
	}

	// Merge daemon socket with any existing user sockets — deduplicated, daemon first.
	daemonSock := expandHomePath(daemonSocketPath)
	seen := map[string]bool{daemonSock: true}
	sockets := []interface{}{daemonSock}
	for _, s := range existingSockets {
		if !seen[s] {
			seen[s] = true
			sockets = append(sockets, s)
		}
	}

	// Build network section: always includes unix sockets; add allowedDomains if configured.
	network := map[string]interface{}{
		"allowUnixSockets": sockets,
	}
	if len(allowedDomains) > 0 {
		network["allowedDomains"] = toIfaceSlice(allowedDomains)
	}

	fs := map[string]interface{}{
		"allowWrite": toIfaceSlice(allowWrite),
		"denyRead":   toIfaceSlice(denyRead),
	}
	if len(denyWrite) > 0 {
		fs["denyWrite"] = toIfaceSlice(denyWrite)
	}
	if len(allowRead) > 0 {
		fs["allowRead"] = toIfaceSlice(allowRead)
	}

	section := map[string]interface{}{
		"enabled":                  true,
		"failIfUnavailable":        true,
		"allowUnsandboxedCommands": false,
		"network":                  network,
		"filesystem":               fs,
	}
	if autoAllowBash != nil {
		section["autoAllowBashIfSandboxed"] = *autoAllowBash
	}
	return section
}

// extractExistingSockets reads network.allowUnixSockets from the existing sandbox
// section of settings, so they can be preserved when the section is rewritten.
func extractExistingSockets(settings map[string]interface{}) []string {
	sandbox, ok := settings["sandbox"].(map[string]interface{})
	if !ok {
		return nil
	}
	network, ok := sandbox["network"].(map[string]interface{})
	if !ok {
		return nil
	}
	raw, ok := network["allowUnixSockets"].([]interface{})
	if !ok {
		return nil
	}
	sockets := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			sockets = append(sockets, s)
		}
	}
	return sockets
}

// appendPermsDenyEntries appends permissions.deny entries to the deny list,
// deduplicating against what's already present. Entries are written as-is
// (callers must pre-expand ~ before calling).
func appendPermsDenyEntries(denySlice []interface{}, entries []string) []interface{} {
	existing := make(map[string]struct{}, len(denySlice))
	for _, v := range denySlice {
		if s, ok := v.(string); ok {
			existing[s] = struct{}{}
		}
	}
	for _, e := range entries {
		if _, ok := existing[e]; !ok {
			denySlice = append(denySlice, e)
			existing[e] = struct{}{}
		}
	}
	return denySlice
}

// collectProjectGitDirs returns deduplicated .git directories for all registered projects.
func collectProjectGitDirs() []string {
	storePath := config.ResolveProjectsPath()
	store := project.NewStore(storePath)
	projects, err := store.List(false)
	if err != nil {
		log.Printf("[sync] warning: failed to load projects for sandbox git dirs: %v", err)
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
	gitDir := filepath.Join(projectPath, ".git")
	return gitDir
}

// expandHomePath expands ~ in path to the user's home directory.
func expandHomePath(p string) string {
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return home
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		return filepath.Join(home, p[2:])
	}
	return p
}

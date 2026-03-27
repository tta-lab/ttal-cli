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

// secretPaths are the paths whose reads should be denied in settings.json.
// These cover the most common secret locations on macOS and Linux.
var secretPaths = []string{
	"~/.config/ttal/.env",
	"~/.ssh",
	"~/.gnupg",
	"~/.netrc",
	"~/.aws/credentials",
	"~/.kube/config",
}

// SyncSandbox updates ~/.claude/settings.json with sandbox and secret-deny config.
// It reads the project store for paths and .git dirs, loads sandbox.toml for
// shared/worker/manager extra paths, and merges into the existing settings.json.
func SyncSandbox(dryRun bool) (SandboxResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return SandboxResult{}, fmt.Errorf("cannot determine home directory: %w", err)
	}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	return syncSandbox(dryRun, settingsPath)
}

func syncSandbox(dryRun bool, settingsPath string) (SandboxResult, error) {
	sandbox := config.LoadSandbox()
	if !sandbox.Enabled {
		return SandboxResult{}, nil
	}

	allowWrite, gitDirCount := buildAllowWritePaths(sandbox)
	denyRead := buildDenyReadPaths()

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
	settings["sandbox"] = buildSandboxSection(allowWrite, denyRead, existingSockets)

	// Append Read deny entries for secrets (additive, preserve existing).
	perms, denySlice, err := extractPermsDenyList(settings)
	if err != nil {
		return result, err
	}
	denySlice = appendSecretDenyEntries(denySlice, denyRead)
	perms["deny"] = denySlice
	settings["permissions"] = perms

	if err := writeSettingsJSON(settingsPath, settings); err != nil {
		return result, fmt.Errorf("writing settings.json: %w", err)
	}

	return result, nil
}

// buildAllowWritePaths collects all paths that should be in allowWrite:
// - :rw paths from sandbox.toml (all planes combined, raw — no existence filtering)
// - .git dirs for all registered projects (deduplicated)
func buildAllowWritePaths(sandbox *config.SandboxConfig) ([]string, int) {
	seen := make(map[string]bool)
	var paths []string

	// sandbox.toml :rw paths (all planes, no existence filtering — declarative config)
	for _, p := range allSandboxPaths(sandbox) {
		bare := stripSuffix(p)
		if strings.HasSuffix(p, ":rw") {
			expanded := expandHomePath(bare)
			if !seen[expanded] {
				seen[expanded] = true
				paths = append(paths, expanded)
			}
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

// buildDenyReadPaths returns the expanded list of secret paths to deny reads for.
func buildDenyReadPaths() []string {
	paths := make([]string, 0, len(secretPaths))
	for _, p := range secretPaths {
		paths = append(paths, expandHomePath(p))
	}
	return paths
}

// daemonSocketPath is the unix socket used for agent↔daemon communication.
// Always whitelisted so that ttal send/go work in all agent sessions.
const daemonSocketPath = "~/.ttal/daemon.sock"

// buildSandboxSection constructs the full sandbox object for settings.json.
// Enforcement settings (failIfUnavailable, allowUnsandboxedCommands) are hardcoded
// secure defaults — they are not user-configurable.
// existingSockets are user-defined unix sockets from a prior settings.json; they
// are preserved and our daemonSocketPath is appended (deduplicated).
func buildSandboxSection(allowWrite, denyRead []string, existingSockets []string) map[string]interface{} {
	aw := make([]interface{}, len(allowWrite))
	for i, p := range allowWrite {
		aw[i] = p
	}
	dr := make([]interface{}, len(denyRead))
	for i, p := range denyRead {
		dr[i] = p
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

	return map[string]interface{}{
		"enabled":                  true,
		"failIfUnavailable":        true,
		"allowUnsandboxedCommands": false,
		"network": map[string]interface{}{
			"allowUnixSockets": sockets,
		},
		"filesystem": map[string]interface{}{
			"allowWrite": aw,
			"denyRead":   dr,
		},
	}
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

// appendSecretDenyEntries appends Read(<path>) entries to the deny list for each
// secret path, using ** glob for directory secrets. Deduplicates against existing entries.
func appendSecretDenyEntries(denySlice []interface{}, denyRead []string) []interface{} {
	existing := make(map[string]struct{}, len(denySlice))
	for _, v := range denySlice {
		if s, ok := v.(string); ok {
			existing[s] = struct{}{}
		}
	}

	for _, p := range denyRead {
		// Use ** glob for directories, bare path for files.
		entry := buildReadDenyEntry(p)
		if _, ok := existing[entry]; !ok {
			denySlice = append(denySlice, entry)
			existing[entry] = struct{}{}
		}
	}
	return denySlice
}

// buildReadDenyEntry constructs the Read() deny entry for a path.
// Paths that look like specific files use bare path; everything else gets /** glob.
func buildReadDenyEntry(p string) string {
	base := filepath.Base(p)
	// Known filenames without extensions are files, not dirs.
	switch base {
	case ".env", "credentials", "config", ".netrc":
		return fmt.Sprintf("Read(%s)", p)
	default:
		return fmt.Sprintf("Read(%s/**)", p)
	}
}

// allSandboxPaths returns the raw ExtraPaths from all planes of sandbox.toml,
// without filtering by existence. Used for declarative settings.json config.
func allSandboxPaths(s *config.SandboxConfig) []string {
	all := make([]string, 0, len(s.Shared.ExtraPaths)+len(s.Worker.ExtraPaths)+len(s.Manager.ExtraPaths))
	all = append(all, s.Shared.ExtraPaths...)
	all = append(all, s.Worker.ExtraPaths...)
	all = append(all, s.Manager.ExtraPaths...)
	return all
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

// stripSuffix removes :ro or :rw suffix from a sandbox path entry.
func stripSuffix(p string) string {
	p = strings.TrimSuffix(p, ":rw")
	p = strings.TrimSuffix(p, ":ro")
	return p
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

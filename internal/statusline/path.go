package statusline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// CompactPath returns a compact representation of cwd for display in the statusline.
// If cwd matches a registered ttal project, returns "(alias)" or "(alias - jobID)".
// Otherwise abbreviates intermediate path components to their first character.
// jobID should be the value of TTAL_JOB_ID (empty if not in a worker session).
func CompactPath(cwd, jobID string) string {
	store := project.NewStore(config.ResolveProjectsPath())
	worktreesRoot := config.WorktreesRoot()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[statusline] warning: cannot determine home dir: %v\n", err)
	}
	return compactPathWith(cwd, jobID, store, worktreesRoot, homeDir)
}

// compactPathWith is the testable variant of CompactPath that accepts explicit
// dependencies instead of reading them from config or the environment.
func compactPathWith(cwd, jobID string, store *project.Store, worktreesRoot, homeDir string) string {
	if cwd == "" {
		return ""
	}

	// 1. Try to resolve a project alias
	alias := project.ResolveProjectAliasWithStore(cwd, store, worktreesRoot)
	if alias != "" {
		if jobID != "" {
			return "(" + alias + " - " + jobID + ")"
		}
		return "(" + alias + ")"
	}

	// 2. Fallback: abbreviate intermediate path segments
	return abbreviatePath(cwd, homeDir)
}

// abbreviatePath collapses intermediate path segments to their first character.
// e.g. /Users/neil/Code/guion-opensource/ttal-cli → ~/C/g/ttal-cli
func abbreviatePath(path, homeDir string) string {
	// Normalise
	clean := filepath.Clean(path)

	// Replace home prefix with ~
	if homeDir != "" {
		cleanHome := filepath.Clean(homeDir)
		if clean == cleanHome {
			return "~"
		}
		if strings.HasPrefix(clean, cleanHome+string(filepath.Separator)) {
			clean = "~" + clean[len(cleanHome):]
		}
	}

	// Split into segments (handle both ~ prefix and absolute paths)
	var prefix string
	var rest string
	if strings.HasPrefix(clean, "~") {
		prefix = "~"
		rest = strings.TrimPrefix(clean, "~")
	} else {
		prefix = ""
		rest = clean
	}

	// rest starts with "/" (or is empty if cwd was exactly homeDir, already handled)
	if rest == "" {
		return prefix
	}

	// Trim leading separator and split
	rest = strings.TrimPrefix(rest, string(filepath.Separator))
	parts := strings.Split(rest, string(filepath.Separator))

	if len(parts) <= 1 {
		// Single component: ~/myproject — nothing to abbreviate
		return prefix + string(filepath.Separator) + parts[0]
	}

	// Abbreviate all intermediate segments (everything except the last)
	abbreviated := make([]string, len(parts))
	for i, p := range parts {
		if i == len(parts)-1 {
			abbreviated[i] = p
		} else {
			if len(p) > 0 {
				abbreviated[i] = string([]rune(p)[:1])
			} else {
				abbreviated[i] = p
			}
		}
	}

	return prefix + string(filepath.Separator) + strings.Join(abbreviated, string(filepath.Separator))
}

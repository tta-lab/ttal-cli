package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// GlobalPromptResult tracks a single global prompt deployment for reporting.
type GlobalPromptResult struct {
	Source  string
	Dest    string
	Runtime string
}

// DeployGlobalPrompt copies one canonical global prompt markdown file into runtime paths.
// Always targets Claude Code (~/.claude/CLAUDE.md), and additionally targets Codex
// (~/.codex/AGENTS.md) when ~/.codex exists and is a directory.
func DeployGlobalPrompt(rawPath string, dryRun bool) ([]GlobalPromptResult, error) {
	source := config.ExpandHome(rawPath)

	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("global prompt file not found: %s", source)
		}
		return nil, fmt.Errorf("checking global prompt file %s: %w", source, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("global prompt path is a directory: %s", source)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	destinations, err := promptDestinations(home)
	if err != nil {
		return nil, err
	}

	results := make([]GlobalPromptResult, 0, len(destinations))
	for _, d := range destinations {
		dest := d.path
		results = append(results, GlobalPromptResult{Source: source, Dest: dest, Runtime: d.runtime})
		if dryRun {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return nil, fmt.Errorf("creating destination dir for %s: %w", dest, err)
		}
		if err := copyPrompt(source, dest); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// DeployGlobalPromptTo copies the global prompt to a single destination path.
// Used for deploying to custom .claude dirs.
func DeployGlobalPromptTo(rawPath string, dest string, dryRun bool) error {
	source := config.ExpandHome(rawPath)
	if _, err := os.Stat(source); err != nil {
		return fmt.Errorf("global prompt not found: %s", source)
	}
	if dryRun {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	return copyPrompt(source, dest)
}

type promptDestination struct {
	path    string
	runtime string
}

func promptDestinations(home string) ([]promptDestination, error) {
	destinations := []promptDestination{
		{path: filepath.Join(home, ".claude", "CLAUDE.md"), runtime: "claude-code"},
	}

	codexDir := filepath.Join(home, ".codex")
	codexInfo, err := os.Stat(codexDir)
	if os.IsNotExist(err) {
		return destinations, nil
	}
	if err != nil {
		return nil, fmt.Errorf("checking codex dir %s: %w", codexDir, err)
	}
	if codexInfo.IsDir() {
		destinations = append(destinations, promptDestination{
			path:    filepath.Join(codexDir, "AGENTS.md"),
			runtime: "codex",
		})
	}

	return destinations, nil
}

func copyPrompt(src, dest string) error {
	if info, err := os.Lstat(dest); err == nil {
		if info.IsDir() {
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("removing existing dir %s: %w", dest, err)
			}
		} else {
			// covers symlinks and regular files
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("removing existing %s: %w", dest, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking existing %s: %w", dest, err)
	}
	return copyFile(src, dest)
}

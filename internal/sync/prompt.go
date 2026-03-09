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
	// Remove existing (symlink or file)
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("removing existing %s: %w", dest, err)
			}
		}
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

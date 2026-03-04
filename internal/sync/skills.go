package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// SkillResult tracks a single skill deployment for reporting.
type SkillResult struct {
	Source    string
	Name      string
	Dest      string // CC destination (~/.claude/skills/)
	CodexDest string // Codex destination (~/.codex/skills/)
}

// DeploySkills symlinks skill directories (those containing SKILL.md) to
// ~/.claude/skills/ (CC + OpenCode) and ~/.codex/skills/ (Codex).
func DeploySkills(skillsPaths []string, dryRun bool) ([]SkillResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	ccDir := filepath.Join(home, ".claude", "skills")
	codexDir := filepath.Join(home, ".codex", "skills")

	var results []SkillResult

	for _, rawPath := range skillsPaths {
		deployed, err := deploySkillsFromDir(rawPath, ccDir, codexDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}

	return results, nil
}

func deploySkillsFromDir(rawPath, ccDir, codexDir string, dryRun bool) ([]SkillResult, error) {
	dir := config.ExpandHome(rawPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: skills path not found: %s\n", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills dir %s: %w", dir, err)
	}

	if !dryRun {
		for _, d := range []string{ccDir, codexDir} {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return nil, fmt.Errorf("creating skills dir %s: %w", d, err)
			}
		}
	}

	results := make([]SkillResult, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillDir := filepath.Join(dir, entry.Name())
		skillMD := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			continue
		}

		ccDest := filepath.Join(ccDir, entry.Name())
		codexDest := filepath.Join(codexDir, entry.Name())
		results = append(results, SkillResult{
			Source:    skillDir,
			Name:      entry.Name(),
			Dest:      ccDest,
			CodexDest: codexDest,
		})

		if dryRun {
			continue
		}

		if err := symlinkSkill(skillDir, ccDest); err != nil {
			return nil, err
		}
		if err := symlinkSkill(skillDir, codexDest); err != nil {
			return nil, err
		}
	}

	return results, nil
}

func symlinkSkill(src, dest string) error {
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("removing existing symlink %s: %w", dest, err)
			}
		} else if info.IsDir() {
			fmt.Fprintf(os.Stderr, "warning: %s is a real directory, skipping\n", dest)
			return nil
		}
	}

	if err := os.Symlink(src, dest); err != nil {
		return fmt.Errorf("creating symlink %s → %s: %w", dest, src, err)
	}
	return nil
}

// CleanSkills removes symlinks in ~/.claude/skills/ and ~/.codex/skills/ that
// point to directories no longer present in any skills_paths source.
func CleanSkills(skillsPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	validSources, err := collectValidSkillSources(skillsPaths)
	if err != nil {
		return nil, err
	}

	destDirs := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
	}

	var removed []string
	for _, destDir := range destDirs {
		entries, err := os.ReadDir(destDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, entry := range entries {
			path, shouldRemove := checkStaleSkillSymlink(destDir, entry, validSources)
			if !shouldRemove {
				continue
			}
			removed = append(removed, path)
			if !dryRun {
				if err := os.Remove(path); err != nil {
					return nil, fmt.Errorf("removing stale skill symlink %s: %w", path, err)
				}
			}
		}
	}

	return removed, nil
}

func collectValidSkillSources(skillsPaths []string) (map[string]bool, error) {
	validSources := make(map[string]bool)
	for _, rawPath := range skillsPaths {
		dir := config.ExpandHome(rawPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading source dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(dir, entry.Name())
			skillMD := filepath.Join(skillDir, "SKILL.md")
			if _, err := os.Stat(skillMD); err == nil {
				validSources[skillDir] = true
			}
		}
	}
	return validSources, nil
}

func checkStaleSkillSymlink(destDir string, entry os.DirEntry, validSources map[string]bool) (string, bool) {
	dest := filepath.Join(destDir, entry.Name())
	info, err := os.Lstat(dest)
	if err != nil {
		return "", false
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", false
	}
	target, err := os.Readlink(dest)
	if err != nil {
		return "", false
	}
	return dest, !validSources[target]
}

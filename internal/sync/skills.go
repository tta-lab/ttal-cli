package sync

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

// SkillResult tracks a single skill deployment for reporting.
type SkillResult struct {
	Source string
	Name   string
	Dest   string
}

// DeploySkills symlinks skill directories (those containing SKILL.md) to ~/.claude/skills/.
func DeploySkills(skillsPaths []string, dryRun bool) ([]SkillResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(home, ".claude", "skills")

	var results []SkillResult

	for _, rawPath := range skillsPaths {
		deployed, err := deploySkillsFromDir(rawPath, destDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}

	return results, nil
}

func deploySkillsFromDir(rawPath, destDir string, dryRun bool) ([]SkillResult, error) {
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
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating skills dir: %w", err)
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

		dest := filepath.Join(destDir, entry.Name())
		results = append(results, SkillResult{Source: skillDir, Name: entry.Name(), Dest: dest})

		if dryRun {
			continue
		}

		if err := symlinkSkill(skillDir, dest); err != nil {
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

// CleanSkills removes symlinks in ~/.claude/skills/ that point to directories
// no longer present in any skills_paths source.
func CleanSkills(skillsPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	destDir := filepath.Join(home, ".claude", "skills")
	validSources, err := collectValidSkillSources(skillsPaths)
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(destDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	removed := make([]string, 0, len(entries))
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

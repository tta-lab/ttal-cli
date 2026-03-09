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

// DeploySkills copies skill directories (those containing SKILL.md) to
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

		if err := copySkillDir(skillDir, ccDest); err != nil {
			return nil, err
		}
		if err := copySkillDir(skillDir, codexDest); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// copySkillDir recursively copies a skill directory to dest.
// If dest exists, it is removed first to ensure a clean copy.
func copySkillDir(src, dest string) error {
	// Remove existing (symlink or directory)
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("removing existing symlink %s: %w", dest, err)
			}
		} else if info.IsDir() {
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("removing existing dir %s: %w", dest, err)
			}
		}
	}
	return copyDir(src, dest)
}

// copyDir recursively copies src directory to dest.
func copyDir(src, dest string) error {
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		destPath := filepath.Join(dest, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, destPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(destPath, data, 0o644); err != nil {
				return err
			}
		}
	}
	return nil
}

// CleanSkills removes directories in ~/.claude/skills/ and ~/.codex/skills/ that
// no longer correspond to any skill in any skills_paths source.
func CleanSkills(skillsPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	validNames, err := collectValidSkillNames(skillsPaths)
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
			path, shouldRemove := checkStaleSkill(destDir, entry, validNames)
			if !shouldRemove {
				continue
			}
			removed = append(removed, path)
			if !dryRun {
				if err := os.RemoveAll(path); err != nil {
					return nil, fmt.Errorf("removing stale skill %s: %w", path, err)
				}
			}
		}
	}

	return removed, nil
}

func collectValidSkillNames(skillsPaths []string) (map[string]bool, error) {
	validNames := make(map[string]bool)
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
			skillMD := filepath.Join(dir, entry.Name(), "SKILL.md")
			if _, err := os.Stat(skillMD); err == nil {
				validNames[entry.Name()] = true
			}
		}
	}
	return validNames, nil
}

func checkStaleSkill(destDir string, entry os.DirEntry, validNames map[string]bool) (string, bool) {
	if !entry.IsDir() {
		return "", false
	}
	dest := filepath.Join(destDir, entry.Name())
	return dest, !validNames[entry.Name()]
}

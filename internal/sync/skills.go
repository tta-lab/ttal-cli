package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// DeploySkillsTo copies skill directories to a custom CC skills dir.
// Used for deploying to custom .claude dirs.
func DeploySkillsTo(skillsPaths []string, ccDir string, dryRun bool) ([]SkillResult, error) {
	var results []SkillResult
	for _, rawPath := range skillsPaths {
		deployed, err := deploySkillsToDir(rawPath, ccDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}
	return results, nil
}

// deployFlatSkillToDir deploys a flat .md file to a single destination.
func deployFlatSkillToDir(srcPath, destDir string, dryRun bool) ([]SkillResult, error) {
	name := strings.TrimSuffix(filepath.Base(srcPath), ".md")
	result := SkillResult{
		Source: srcPath,
		Name:   name,
		Dest:   destDir,
	}

	if dryRun {
		return []SkillResult{result}, nil
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, fmt.Errorf("creating skill dir %s: %w", destDir, err)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading skill file %s: %w", srcPath, err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "SKILL.md"), data, 0o644); err != nil {
		return nil, fmt.Errorf("writing SKILL.md: %w", err)
	}
	return []SkillResult{result}, nil
}

// deploySkillsToDir deploys skills from a single source path to a single destination dir.
func deploySkillsToDir(rawPath, destDir string, dryRun bool) ([]SkillResult, error) {
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
			return nil, fmt.Errorf("creating skills dir %s: %w", destDir, err)
		}
	}

	results := make([]SkillResult, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			// Handle flat .md files (e.g., ttal-voice.md -> skill "ttal-voice")
			if strings.HasSuffix(entry.Name(), ".md") {
				srcPath := filepath.Join(dir, entry.Name())
				ccDest := filepath.Join(destDir, strings.TrimSuffix(entry.Name(), ".md"))
				deployed, err := deployFlatSkillToDir(srcPath, ccDest, dryRun)
				if err != nil {
					return nil, err
				}
				results = append(results, deployed...)
			}
			continue
		}
		skillDir := filepath.Join(dir, entry.Name())
		skillMD := filepath.Join(skillDir, "SKILL.md")
		if _, err := os.Stat(skillMD); err != nil {
			continue
		}
		ccDest := filepath.Join(destDir, entry.Name())
		results = append(results, SkillResult{
			Source: skillDir,
			Name:   entry.Name(),
			Dest:   ccDest,
		})
		if dryRun {
			continue
		}
		if err := deploySkill(skillDir, ccDest); err != nil {
			return nil, err
		}
	}
	return results, nil
}

// deployFlatSkill deploys a flat .md file as a skill by creating subdirs with SKILL.md.
func deployFlatSkill(srcPath, ccDest, codexDest string, dryRun bool) ([]SkillResult, error) {
	name := strings.TrimSuffix(filepath.Base(srcPath), ".md")
	result := SkillResult{
		Source:    srcPath,
		Name:      name,
		Dest:      ccDest,
		CodexDest: codexDest,
	}

	if dryRun {
		return []SkillResult{result}, nil
	}

	for _, d := range []string{ccDest, codexDest} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, fmt.Errorf("creating skill dir %s: %w", d, err)
		}
		data, err := os.ReadFile(srcPath)
		if err != nil {
			return nil, fmt.Errorf("reading skill file %s: %w", srcPath, err)
		}
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), data, 0o644); err != nil {
			return nil, fmt.Errorf("writing SKILL.md: %w", err)
		}
	}
	return []SkillResult{result}, nil
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
			// Handle flat .md files (e.g., ttal-voice.md -> skill "ttal-voice")
			if strings.HasSuffix(entry.Name(), ".md") {
				srcPath := filepath.Join(dir, entry.Name())
				ccDest := filepath.Join(ccDir, strings.TrimSuffix(entry.Name(), ".md"))
				codexDest := filepath.Join(codexDir, strings.TrimSuffix(entry.Name(), ".md"))
				deployed, err := deployFlatSkill(srcPath, ccDest, codexDest, dryRun)
				if err != nil {
					return nil, err
				}
				results = append(results, deployed...)
			}
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

		if err := deploySkill(skillDir, ccDest); err != nil {
			return nil, err
		}
		if err := deploySkill(skillDir, codexDest); err != nil {
			return nil, err
		}
	}

	return results, nil
}

// deploySkill copies only SKILL.md from a skill directory to dest.
// If dest exists, it is removed first to ensure a clean copy.
func deploySkill(src, dest string) error {
	if info, err := os.Lstat(dest); err == nil {
		if info.IsDir() {
			if err := os.RemoveAll(dest); err != nil {
				return fmt.Errorf("removing existing dir %s: %w", dest, err)
			}
		} else {
			if err := os.Remove(dest); err != nil {
				return fmt.Errorf("removing existing %s: %w", dest, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("checking existing %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("creating dir %s: %w", dest, err)
	}
	return copyFile(filepath.Join(src, "SKILL.md"), filepath.Join(dest, "SKILL.md"))
}

// copyFile copies a single file from src to dest.
func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
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

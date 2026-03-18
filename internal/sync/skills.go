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
	Source           string
	Name             string
	Dest             string // CC destination (~/.claude/skills/)
	CodexDest        string // Codex destination (~/.codex/skills/)
	AgentsSkillsDest string // .agents/skills destination
}

// DeploySkills copies skill directories (those containing SKILL.md) to
// ~/.claude/skills/ (CC), ~/.codex/skills/ (Codex), and ~/.agents/skills (unified).
func DeploySkills(skillsPaths []string, dryRun bool) ([]SkillResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	targetDirs := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}

	if err := ensureTargetDirs(targetDirs, dryRun); err != nil {
		return nil, err
	}

	var results []SkillResult
	for _, rawPath := range skillsPaths {
		deployed, err := deploySkillsFromDir(rawPath, targetDirs, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}
	return results, nil
}

func ensureTargetDirs(dirs []string, dryRun bool) error {
	if dryRun {
		return nil
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return fmt.Errorf("creating skills dir %s: %w", d, err)
		}
	}
	return nil
}

func deploySkillsFromDir(rawPath string, targetDirs []string, dryRun bool) ([]SkillResult, error) {
	dir := config.ExpandHome(rawPath)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: skills path not found: %s\n", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading skills dir %s: %w", dir, err)
	}

	var results []SkillResult
	for _, entry := range entries {
		switch {
		case isFlatSkill(entry):
			r, err := deployFromFile(dir, entry.Name(), targetDirs, dryRun)
			if err != nil {
				return nil, err
			}
			results = append(results, r...)
		case isSkillDir(dir, entry):
			r, err := deployFromDir(dir, entry.Name(), targetDirs, dryRun)
			if err != nil {
				return nil, err
			}
			results = append(results, r)
		}
	}
	return results, nil
}

func isFlatSkill(entry os.DirEntry) bool {
	return !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md")
}

func isSkillDir(dir string, entry os.DirEntry) bool {
	if !entry.IsDir() {
		return false
	}
	_, err := os.Stat(filepath.Join(dir, entry.Name(), "SKILL.md"))
	return err == nil
}

func deployFromFile(dir, name string, targetDirs []string, dryRun bool) ([]SkillResult, error) {
	srcPath := filepath.Join(dir, name)
	skillName := strings.TrimSuffix(name, ".md")

	ccDest := filepath.Join(targetDirs[0], skillName)
	codexDest := filepath.Join(targetDirs[1], skillName)
	agentsDest := filepath.Join(targetDirs[2], skillName)

	result := SkillResult{
		Source:           srcPath,
		Name:             skillName,
		Dest:             ccDest,
		CodexDest:        codexDest,
		AgentsSkillsDest: agentsDest,
	}
	if dryRun {
		return []SkillResult{result}, nil
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading skill file %s: %w", srcPath, err)
	}

	for _, dest := range []string{ccDest, codexDest, agentsDest} {
		if err := writeSkill(filepath.Join(dest, "SKILL.md"), data); err != nil {
			return nil, err
		}
	}
	return []SkillResult{result}, nil
}

func deployFromDir(dir, name string, targetDirs []string, dryRun bool) (SkillResult, error) {
	srcDir := filepath.Join(dir, name)

	ccDest := filepath.Join(targetDirs[0], name)
	codexDest := filepath.Join(targetDirs[1], name)
	agentsDest := filepath.Join(targetDirs[2], name)

	result := SkillResult{
		Source:           srcDir,
		Name:             name,
		Dest:             ccDest,
		CodexDest:        codexDest,
		AgentsSkillsDest: agentsDest,
	}
	if dryRun {
		return result, nil
	}

	for _, dest := range []string{ccDest, codexDest, agentsDest} {
		if err := deploySkillToDir(dest, srcDir); err != nil {
			return SkillResult{}, err
		}
	}
	return result, nil
}

// deploySkillToDir copies SKILL.md from srcDir to dest.
// If dest exists, it is removed first to ensure a clean copy.
func deploySkillToDir(dest, srcDir string) error {
	if err := os.RemoveAll(dest); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing %s: %w", dest, err)
	}
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return fmt.Errorf("creating dir %s: %w", dest, err)
	}
	return copyFile(filepath.Join(srcDir, "SKILL.md"), filepath.Join(dest, "SKILL.md"))
}

func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("reading %s: %w", src, err)
	}
	return writeSkill(dest, data)
}

func writeSkill(dest string, data []byte) error {
	parent := filepath.Dir(dest)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return fmt.Errorf("creating skill dir %s: %w", parent, err)
	}
	if err := os.WriteFile(dest, data, 0o644); err != nil {
		return fmt.Errorf("writing SKILL.md: %w", err)
	}
	return nil
}

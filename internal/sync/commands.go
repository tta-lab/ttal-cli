package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

// CommandResult tracks a single command deployment for reporting.
type CommandResult struct {
	Source string
	Name   string
	CCDest string
	OCDest string
}

// DeployCommands reads canonical command .md files from the given paths and deploys
// runtime-specific variants to Claude Code (as skills) and OpenCode (as commands).
func DeployCommands(commandsPaths []string, dryRun bool) ([]CommandResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	ccSkillsDir := filepath.Join(home, ".claude", "skills")
	ocCmdsDir := filepath.Join(home, ".config", "opencode", "commands")

	if !dryRun {
		if err := os.MkdirAll(ccSkillsDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating CC skills dir: %w", err)
		}
		if err := os.MkdirAll(ocCmdsDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating OC commands dir: %w", err)
		}
	}

	var results []CommandResult
	for _, rawPath := range commandsPaths {
		deployed, err := deployCommandsFromDir(rawPath, ccSkillsDir, ocCmdsDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}
	return results, nil
}

func deployCommandsFromDir(rawPath, ccSkillsDir, ocCmdsDir string, dryRun bool) ([]CommandResult, error) {
	dir := config.ExpandHome(rawPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: commands path not found: %s\n", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("reading commands dir %s: %w", dir, err)
	}

	results := make([]CommandResult, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		r, err := deployOneCommand(filepath.Join(dir, entry.Name()), ccSkillsDir, ocCmdsDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func deployOneCommand(srcPath, ccSkillsDir, ocCmdsDir string, dryRun bool) (CommandResult, error) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return CommandResult{}, fmt.Errorf("reading %s: %w", srcPath, err)
	}

	cmd, err := ParseCommandFile(string(content))
	if err != nil {
		return CommandResult{}, fmt.Errorf("parsing %s: %w", srcPath, err)
	}

	ccContent, err := GenerateCCCommandVariant(cmd)
	if err != nil {
		return CommandResult{}, fmt.Errorf("generating CC variant for %s: %w", cmd.Frontmatter.Name, err)
	}

	ocContent, err := GenerateOCCommandVariant(cmd)
	if err != nil {
		return CommandResult{}, fmt.Errorf("generating OC variant for %s: %w", cmd.Frontmatter.Name, err)
	}

	// CC: create skill directory with SKILL.md inside
	ccSkillDir := filepath.Join(ccSkillsDir, cmd.Frontmatter.Name)
	ccDest := filepath.Join(ccSkillDir, "SKILL.md")

	// OC: flat file in commands dir
	ocDest := filepath.Join(ocCmdsDir, cmd.Frontmatter.Name+".md")

	result := CommandResult{
		Source: srcPath,
		Name:   cmd.Frontmatter.Name,
		CCDest: ccDest,
		OCDest: ocDest,
	}

	if dryRun {
		return result, nil
	}

	if err := os.MkdirAll(ccSkillDir, 0o755); err != nil {
		return CommandResult{}, fmt.Errorf("creating CC skill dir %s: %w", ccSkillDir, err)
	}
	if err := os.WriteFile(ccDest, []byte(ccContent), 0o644); err != nil {
		return CommandResult{}, fmt.Errorf("writing CC command %s: %w", ccDest, err)
	}
	if err := os.WriteFile(ocDest, []byte(ocContent), 0o644); err != nil {
		return CommandResult{}, fmt.Errorf("writing OC command %s: %w", ocDest, err)
	}

	return result, nil
}

// CleanCommands removes ttal-managed command files that no longer exist in source paths.
func CleanCommands(commandsPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	validNames, err := collectValidCommandNames(commandsPaths)
	if err != nil {
		return nil, err
	}

	var removed []string

	ccSkillsDir := filepath.Join(home, ".claude", "skills")
	ccRemoved, err := cleanManagedCommandSkills(ccSkillsDir, validNames, dryRun)
	if err != nil {
		return nil, err
	}
	removed = append(removed, ccRemoved...)

	ocCmdsDir := filepath.Join(home, ".config", "opencode", "commands")
	ocRemoved, err := cleanManagedCommandFiles(ocCmdsDir, validNames, dryRun)
	if err != nil {
		return nil, err
	}
	removed = append(removed, ocRemoved...)

	return removed, nil
}

func collectValidCommandNames(commandsPaths []string) (map[string]bool, error) {
	validNames := make(map[string]bool)
	for _, rawPath := range commandsPaths {
		dir := config.ExpandHome(rawPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading source dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			filePath := filepath.Join(dir, entry.Name())
			content, err := os.ReadFile(filePath)
			if err != nil {
				return nil, fmt.Errorf("reading command file %s for cleanup validation: %w", filePath, err)
			}
			cmd, err := ParseCommandFile(string(content))
			if err != nil {
				return nil, fmt.Errorf("parsing command file %s during cleanup: %w", filePath, err)
			}
			validNames[cmd.Frontmatter.Name] = true
		}
	}
	return validNames, nil
}

// cleanManagedCommandSkills removes CC skill directories that were deployed from commands.
func cleanManagedCommandSkills(skillsDir string, validNames map[string]bool, dryRun bool) ([]string, error) {
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if validNames[name] {
			continue
		}
		skillMD := filepath.Join(skillsDir, name, "SKILL.md")
		content, err := os.ReadFile(skillMD)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s during cleanup: %w", skillMD, err)
		}
		if !strings.Contains(string(content), ManagedMarkerField) {
			continue
		}
		path := filepath.Join(skillsDir, name)
		removed = append(removed, path)
		if !dryRun {
			if err := os.RemoveAll(path); err != nil {
				return nil, fmt.Errorf("removing stale command skill %s: %w", path, err)
			}
		}
	}
	return removed, nil
}

// cleanManagedCommandFiles removes OC command files that were deployed from commands.
func cleanManagedCommandFiles(cmdsDir string, validNames map[string]bool, dryRun bool) ([]string, error) {
	entries, err := os.ReadDir(cmdsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var removed []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if validNames[name] {
			continue
		}
		path := filepath.Join(cmdsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("reading %s during cleanup: %w", path, err)
		}
		if !strings.Contains(string(content), ManagedMarkerField) {
			continue
		}
		removed = append(removed, path)
		if !dryRun {
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("removing stale command %s: %w", path, err)
			}
		}
	}
	return removed, nil
}

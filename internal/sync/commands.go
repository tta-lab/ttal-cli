package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// CommandResult tracks a single command deployment for reporting.
type CommandResult struct {
	Source           string
	Name             string
	CCDest           string
	CodexDest        string
	AgentsSkillsDest string // .agents/skills deployment
}

// DeployCommands reads canonical command .md files from the given paths and deploys
// runtime-specific variants to Claude Code and Codex.
// Also deploys to .agents/skills for unified skills support.
func DeployCommands(commandsPaths []string, dryRun bool) ([]CommandResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	baseDirs := []string{
		filepath.Join(home, ".claude", "skills"),
		filepath.Join(home, ".codex", "skills"),
		filepath.Join(home, ".agents", "skills"),
	}

	if !dryRun {
		for _, d := range baseDirs {
			if err := os.MkdirAll(d, 0o755); err != nil {
				return nil, fmt.Errorf("creating dir %s: %w", d, err)
			}
		}
	}

	var results []CommandResult
	for _, rawPath := range commandsPaths {
		deployed, err := deployCommandsFromDir(rawPath, baseDirs, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
	}
	return results, nil
}

func deployCommandsFromDir(rawPath string, baseDirs []string, dryRun bool) ([]CommandResult, error) {
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
		r, err := deployOneCommand(filepath.Join(dir, entry.Name()), baseDirs, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

func deployOneCommand(srcPath string, baseDirs []string, dryRun bool) (CommandResult, error) {
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

	ccDir := baseDirs[0]
	codexDir := baseDirs[1]
	agentsDir := baseDirs[2]

	result := CommandResult{
		Source:           srcPath,
		Name:             cmd.Frontmatter.Name,
		CCDest:           filepath.Join(ccDir, cmd.Frontmatter.Name, "SKILL.md"),
		CodexDest:        filepath.Join(codexDir, cmd.Frontmatter.Name, "SKILL.md"),
		AgentsSkillsDest: filepath.Join(agentsDir, cmd.Frontmatter.Name, "SKILL.md"),
	}

	if dryRun {
		return result, nil
	}

	// Deploy to all targets using CC content
	for _, base := range []string{ccDir, codexDir, agentsDir} {
		destDir := filepath.Join(base, cmd.Frontmatter.Name)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return CommandResult{}, fmt.Errorf("creating dir %s: %w", destDir, err)
		}
		dest := filepath.Join(destDir, "SKILL.md")
		if err := os.WriteFile(dest, []byte(ccContent), 0o644); err != nil {
			return CommandResult{}, fmt.Errorf("writing %s: %w", dest, err)
		}
	}

	return result, nil
}

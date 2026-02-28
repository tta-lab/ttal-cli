package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/config"
)

// AgentResult tracks a single agent deployment for reporting.
type AgentResult struct {
	Source    string
	Name      string
	CCDest    string
	OCDest    string
	CodexDest string
}

// DeployAgents reads canonical agent .md files from the given paths and deploys
// runtime-specific variants to Claude Code, OpenCode, and Codex agent directories.
func DeployAgents(subagentsPaths []string, dryRun bool) ([]AgentResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	ccDir := filepath.Join(home, ".claude", "agents")
	ocDir := filepath.Join(home, ".config", "opencode", "agents")
	codexDir := filepath.Join(home, ".codex", "agents")

	if !dryRun {
		if err := os.MkdirAll(ccDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating CC agents dir: %w", err)
		}
		if err := os.MkdirAll(ocDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating OC agents dir: %w", err)
		}
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating Codex agents dir: %w", err)
		}
	}

	var results []AgentResult
	var allAgents []*ParsedAgent

	for _, rawPath := range subagentsPaths {
		deployed, agents, err := deployAgentsFromDir(rawPath, ccDir, ocDir, codexDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
		allAgents = append(allAgents, agents...)
	}

	// Deploy Codex config.toml registration entries
	if len(allAgents) > 0 {
		if err := DeployCodexAgents(allAgents, dryRun); err != nil {
			return nil, fmt.Errorf("Codex config sync failed: %w", err)
		}
	}

	return results, nil
}

func deployAgentsFromDir(rawPath, ccDir, ocDir, codexDir string, dryRun bool) ([]AgentResult, []*ParsedAgent, error) {
	dir := config.ExpandHome(rawPath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "warning: subagents path not found: %s\n", dir)
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("reading subagents dir %s: %w", dir, err)
	}

	results := make([]AgentResult, 0, len(entries))
	agents := make([]*ParsedAgent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		r, agent, err := deployOneAgent(filepath.Join(dir, entry.Name()), ccDir, ocDir, codexDir, dryRun)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, r)
		agents = append(agents, agent)
	}
	return results, agents, nil
}

func deployOneAgent(srcPath, ccDir, ocDir, codexDir string, dryRun bool) (AgentResult, *ParsedAgent, error) {
	content, err := os.ReadFile(srcPath)
	if err != nil {
		return AgentResult{}, nil, fmt.Errorf("reading %s: %w", srcPath, err)
	}

	agent, err := ParseAgentFile(string(content))
	if err != nil {
		return AgentResult{}, nil, fmt.Errorf("parsing %s: %w", srcPath, err)
	}

	ccContent, err := GenerateCCVariant(agent)
	if err != nil {
		return AgentResult{}, nil, fmt.Errorf("generating CC variant for %s: %w", agent.Frontmatter.Name, err)
	}

	ocContent, err := GenerateOCVariant(agent)
	if err != nil {
		return AgentResult{}, nil, fmt.Errorf("generating OC variant for %s: %w", agent.Frontmatter.Name, err)
	}

	ccDest := filepath.Join(ccDir, agent.Frontmatter.Name+".md")
	ocDest := filepath.Join(ocDir, agent.Frontmatter.Name+".md")
	codexDest := filepath.Join(codexDir, agent.Frontmatter.Name+".toml")

	result := AgentResult{
		Source:    srcPath,
		Name:      agent.Frontmatter.Name,
		CCDest:    ccDest,
		OCDest:    ocDest,
		CodexDest: codexDest,
	}

	if dryRun {
		return result, agent, nil
	}

	if err := os.WriteFile(ccDest, []byte(ccContent), 0o644); err != nil {
		return AgentResult{}, nil, fmt.Errorf("writing CC agent %s: %w", ccDest, err)
	}
	if err := os.WriteFile(ocDest, []byte(ocContent), 0o644); err != nil {
		return AgentResult{}, nil, fmt.Errorf("writing OC agent %s: %w", ocDest, err)
	}
	// Codex .toml files are written by DeployCodexAgents to avoid duplicate writes

	return result, agent, nil
}

// CleanAgents removes ttal-managed agent files that no longer exist in source paths.
// Only removes files containing the ManagedMarkerField to avoid deleting user-created agents.
// Also cleans stale Codex agent .toml files and config.toml entries.
func CleanAgents(subagentsPaths []string, dryRun bool) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	validNames, err := collectValidAgentNames(subagentsPaths)
	if err != nil {
		return nil, err
	}

	dirs := []string{
		filepath.Join(home, ".claude", "agents"),
		filepath.Join(home, ".config", "opencode", "agents"),
	}

	var removed []string
	for _, dir := range dirs {
		cleaned, err := cleanManagedAgentsInDir(dir, validNames, dryRun)
		if err != nil {
			return nil, err
		}
		removed = append(removed, cleaned...)
	}

	// Clean stale Codex agents
	codexCleaned, err := CleanCodexAgents(validNames, dryRun)
	if err != nil {
		return nil, fmt.Errorf("Codex agent cleanup failed: %w", err)
	}
	removed = append(removed, codexCleaned...)

	return removed, nil
}

func collectValidAgentNames(subagentsPaths []string) (map[string]bool, error) {
	validNames := make(map[string]bool)
	for _, rawPath := range subagentsPaths {
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
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				continue
			}
			agent, err := ParseAgentFile(string(content))
			if err != nil {
				continue
			}
			validNames[agent.Frontmatter.Name] = true
		}
	}
	return validNames, nil
}

func cleanManagedAgentsInDir(dir string, validNames map[string]bool, dryRun bool) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	removed := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".md")
		if validNames[name] {
			continue
		}
		path := filepath.Join(dir, entry.Name())

		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if !strings.Contains(string(content), ManagedMarkerField) {
			continue
		}

		removed = append(removed, path)
		if !dryRun {
			if err := os.Remove(path); err != nil {
				return nil, fmt.Errorf("removing stale agent %s: %w", path, err)
			}
		}
	}

	return removed, nil
}

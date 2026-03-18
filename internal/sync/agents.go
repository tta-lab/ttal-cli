package sync

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// AgentResult tracks a single agent deployment for reporting.
type AgentResult struct {
	Source    string
	Name      string
	CCDest    string
	CodexDest string
}

// DeployAgents reads canonical agent .md files from the given paths and deploys
// runtime-specific variants to Claude Code and Codex agent directories.
func DeployAgents(subagentsPaths []string, dryRun bool) ([]AgentResult, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}

	ccDir := filepath.Join(home, ".claude", "agents")
	codexDir := filepath.Join(home, ".codex", "agents")

	if !dryRun {
		if err := os.MkdirAll(ccDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating CC agents dir: %w", err)
		}
		if err := os.MkdirAll(codexDir, 0o755); err != nil {
			return nil, fmt.Errorf("creating Codex agents dir: %w", err)
		}
	}

	var results []AgentResult
	var allAgents []*ParsedAgent

	for _, rawPath := range subagentsPaths {
		deployed, agents, err := deployAgentsFromDir(rawPath, ccDir, codexDir, dryRun)
		if err != nil {
			return nil, err
		}
		results = append(results, deployed...)
		allAgents = append(allAgents, agents...)
	}

	// Deploy Codex config.toml registration entries
	if len(allAgents) > 0 {
		if err := DeployCodexAgents(allAgents, dryRun); err != nil {
			return nil, fmt.Errorf("codex config sync failed: %w", err)
		}
	}

	return results, nil
}

func deployAgentsFromDir(rawPath, ccDir, codexDir string, dryRun bool) ([]AgentResult, []*ParsedAgent, error) {
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

		// Skip non-agent markdown files
		name := strings.TrimSuffix(entry.Name(), ".md")
		if name == "README" || name == "CLAUDE.user" || name == "CLAUDE" {
			continue
		}

		r, agent, err := deployOneAgent(filepath.Join(dir, entry.Name()), ccDir, codexDir, dryRun)
		if err != nil {
			return nil, nil, err
		}
		results = append(results, r)
		agents = append(agents, agent)
	}
	return results, agents, nil
}

func deployOneAgent(srcPath, ccDir, codexDir string, dryRun bool) (AgentResult, *ParsedAgent, error) {
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

	ccDest := filepath.Join(ccDir, agent.Frontmatter.Name+".md")
	codexDest := filepath.Join(codexDir, agent.Frontmatter.Name+".toml")

	result := AgentResult{
		Source:    srcPath,
		Name:      agent.Frontmatter.Name,
		CCDest:    ccDest,
		CodexDest: codexDest,
	}

	if dryRun {
		return result, agent, nil
	}

	if err := os.WriteFile(ccDest, []byte(ccContent), 0o644); err != nil {
		return AgentResult{}, nil, fmt.Errorf("writing CC agent %s: %w", ccDest, err)
	}
	// Codex .toml files are written by DeployCodexAgents to avoid duplicate writes

	return result, agent, nil
}

// DiscoverTtalAgents scans subagents_paths for .md files with ttal: frontmatter.
// Returns only agents that have a ttal: section, sorted by name.
func DiscoverTtalAgents(subagentsPaths []string) ([]*ParsedAgent, error) {
	var agents []*ParsedAgent
	for _, rawPath := range subagentsPaths {
		dir := config.ExpandHome(rawPath)
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "warning: subagents path not found: %s\n", dir)
				continue
			}
			return nil, fmt.Errorf("reading subagents dir %s: %w", dir, err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", entry.Name(), err)
			}
			agent, err := ParseAgentFile(string(content))
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", entry.Name(), err)
				continue
			}
			if agent.Frontmatter.Ttal != nil {
				agents = append(agents, agent)
			}
		}
	}
	sort.Slice(agents, func(i, j int) bool {
		return agents[i].Frontmatter.Name < agents[j].Frontmatter.Name
	})
	return agents, nil
}

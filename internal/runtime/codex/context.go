package codex

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/config"
	syncer "github.com/tta-lab/ttal-cli/internal/sync"
)

// BuildAgentContext assembles the full system prompt for a Codex agent.
// Combines: agent identity (.md body after frontmatter) + rules.
// Codex agents wake via the unified trigger and run `ttal context` themselves.
// Layout: Part 1 (identity) + Part 2 (rules), separated by "---".
func BuildAgentContext(agentName, teamPath string, _ []string) (string, error) {
	mdPath := filepath.Join(teamPath, agentName, "AGENTS.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read agent identity %s: %w", mdPath, err)
	}
	parsed, err := syncer.ParseAgentFile(string(data))
	if err != nil {
		return "", fmt.Errorf("parse agent identity %s: %w", mdPath, err)
	}

	sections := []string{parsed.Body}

	// Part 2: Rules from rules_paths
	rules := loadRulesContent()
	if rules != "" {
		sections = append(sections, rules)
	}

	return strings.Join(sections, "\n\n---\n\n"), nil
}

// loadRulesContent reads RULE.md files from config.Sync.RulesPaths using a dry-run
// DeployRules call to discover sources, then reads and concatenates their content.
// Returns "" if no rules are found or config cannot be loaded.
func loadRulesContent() string {
	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	if len(cfg.Sync.RulesPaths) == 0 {
		return ""
	}
	results, err := syncer.DeployRules(cfg.Sync.RulesPaths, true)
	if err != nil || len(results) == 0 {
		if len(cfg.Sync.RulesPaths) > 0 {
			log.Printf("[codex] warning: no rules loaded from %v", cfg.Sync.RulesPaths)
		}
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Shared Knowledge\n\n")
	for _, r := range results {
		content, err := os.ReadFile(r.Source)
		if err != nil {
			continue
		}
		sb.WriteString(strings.TrimSpace(string(content)))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// ResolveCodexModel reads the codex-specific model from agent frontmatter.
// Falls back to AdapterConfig.Model if codex section has no model.
func ResolveCodexModel(agentName, teamPath, fallback string) string {
	mdPath := filepath.Join(teamPath, agentName, "AGENTS.md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return fallback
	}
	parsed, err := syncer.ParseAgentFile(string(data))
	if err != nil {
		return fallback
	}
	if parsed.Frontmatter.Codex != nil {
		if m, ok := parsed.Frontmatter.Codex["model"].(string); ok && m != "" {
			return m
		}
	}
	return fallback
}

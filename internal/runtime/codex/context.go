package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/promptrender"
	syncer "github.com/tta-lab/ttal-cli/internal/sync"
)

// BuildAgentContext assembles the full system prompt for a Codex agent.
// Combines: agent identity (.md body after frontmatter) + rendered context template.
func BuildAgentContext(agentName, teamPath string, env []string) (string, error) {
	mdPath := filepath.Join(teamPath, agentName+".md")
	data, err := os.ReadFile(mdPath)
	if err != nil {
		return "", fmt.Errorf("read agent identity %s: %w", mdPath, err)
	}
	parsed, err := syncer.ParseAgentFile(string(data))
	if err != nil {
		return "", fmt.Errorf("parse agent identity %s: %w", mdPath, err)
	}

	cfg, err := config.Load()
	if err != nil {
		return parsed.Body, nil // graceful: identity without context
	}
	tmpl := cfg.Prompt("context")
	if tmpl == "" {
		return parsed.Body, nil
	}
	teamName := cfg.TeamName()
	rendered := promptrender.RenderTemplate(tmpl, agentName, teamName, env)
	if rendered == "" {
		return parsed.Body, nil
	}
	return parsed.Body + "\n\n---\n\n" + rendered, nil
}

// ResolveCodexModel reads the codex-specific model from agent frontmatter.
// Falls back to AdapterConfig.Model if codex section has no model.
func ResolveCodexModel(agentName, teamPath, fallback string) string {
	mdPath := filepath.Join(teamPath, agentName+".md")
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

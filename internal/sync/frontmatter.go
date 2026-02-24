package sync

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ManagedMarker is embedded in deployed files so CleanAgents can identify
// ttal-managed files and avoid deleting user-created ones.
const ManagedMarker = "<!-- managed by ttal sync -->"

// AgentFrontmatter holds parsed frontmatter from a canonical agent .md file.
type AgentFrontmatter struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	ClaudeCode  map[string]interface{} `yaml:"claude-code"`
	OpenCode    map[string]interface{} `yaml:"opencode"`
}

// ParsedAgent holds the parsed frontmatter and body of an agent .md file.
type ParsedAgent struct {
	Frontmatter AgentFrontmatter
	Body        string
}

// ParseAgentFile splits a canonical agent .md file into frontmatter and body.
// Expected format:
//
//	---
//	name: foo
//	...
//	---
//	Body text here
func ParseAgentFile(content string) (*ParsedAgent, error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, fmt.Errorf("missing opening --- delimiter")
	}

	// Find closing delimiter
	rest := content[3:]
	rest = strings.TrimLeft(rest, " \t")
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	} else if len(rest) > 1 && rest[0] == '\r' && rest[1] == '\n' {
		rest = rest[2:]
	}

	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return nil, fmt.Errorf("missing closing --- delimiter")
	}

	yamlContent := rest[:idx]
	body := rest[idx+4:]
	body = strings.TrimLeft(body, "\r\n")

	var fm AgentFrontmatter
	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return nil, fmt.Errorf("invalid YAML frontmatter: %w", err)
	}

	if fm.Name == "" {
		return nil, fmt.Errorf("frontmatter missing required field: name")
	}

	return &ParsedAgent{
		Frontmatter: fm,
		Body:        body,
	}, nil
}

// GenerateCCVariant produces a Claude Code agent .md file from a parsed canonical agent.
// Includes shared fields (name, description) plus claude-code specific fields.
func GenerateCCVariant(agent *ParsedAgent) (string, error) {
	fm := make(map[string]interface{})
	fm["name"] = agent.Frontmatter.Name
	if agent.Frontmatter.Description != "" {
		fm["description"] = agent.Frontmatter.Description
	}
	for k, v := range agent.Frontmatter.ClaudeCode {
		fm[k] = v
	}
	return renderAgentFile(fm, agent.Body)
}

// GenerateOCVariant produces an OpenCode agent .md file from a parsed canonical agent.
// Includes shared fields (name, description) plus opencode specific fields.
func GenerateOCVariant(agent *ParsedAgent) (string, error) {
	fm := make(map[string]interface{})
	fm["name"] = agent.Frontmatter.Name
	if agent.Frontmatter.Description != "" {
		fm["description"] = agent.Frontmatter.Description
	}
	for k, v := range agent.Frontmatter.OpenCode {
		fm[k] = v
	}
	return renderAgentFile(fm, agent.Body)
}

func renderAgentFile(fm map[string]interface{}, body string) (string, error) {
	yamlBytes, err := yaml.Marshal(fm)
	if err != nil {
		return "", fmt.Errorf("failed to marshal frontmatter: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(ManagedMarker)
	sb.WriteString("\n---\n")
	sb.Write(yamlBytes)
	sb.WriteString("---\n")
	if body != "" {
		sb.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			sb.WriteString("\n")
		}
	}
	return sb.String(), nil
}

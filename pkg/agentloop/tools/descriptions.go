package tools

import (
	_ "embed"
	"strings"

	"charm.land/fantasy"
)

//go:embed descriptions/bash.md
var bashDescription string

//go:embed descriptions/read.md
var readDescription string

//go:embed descriptions/read_md.md
var readMDDescription string

//go:embed descriptions/read_url.md
var readURLDescription string

//go:embed descriptions/search_web.md
var searchWebDescription string

//go:embed descriptions/glob.md
var globDescription string

//go:embed descriptions/grep.md
var grepDescription string

// schemaDescription returns the first line of the full description,
// used as the short tool schema description sent to the API.
func schemaDescription(full string) string {
	if i := strings.Index(full, "\n"); i != -1 {
		return strings.TrimSpace(full[:i])
	}
	return strings.TrimSpace(full)
}

// RichToolInfo holds a tool's name and full .md description for system prompt injection.
// Defined here (not in agentloop) to avoid circular import.
type RichToolInfo struct {
	Name        string
	Description string
}

// richDescriptions maps tool name → full .md content (for system prompt injection).
var richDescriptions = map[string]string{
	"bash":       bashDescription,
	"read":       readDescription,
	"read_md":    readMDDescription,
	"read_url":   readURLDescription,
	"search_web": searchWebDescription,
	"glob":       globDescription,
	"grep":       grepDescription,
}

// RichToolDescriptions returns rich descriptions for the given tools.
// Only includes tools that are in the provided slice (matched by name).
// Falls back to the tool's schema description if no .md file is found.
func RichToolDescriptions(allTools []fantasy.AgentTool) []RichToolInfo {
	infos := make([]RichToolInfo, 0, len(allTools))
	for _, t := range allTools {
		name := t.Info().Name
		desc, ok := richDescriptions[name]
		if !ok {
			desc = t.Info().Description // fallback to schema description
		}
		infos = append(infos, RichToolInfo{
			Name:        name,
			Description: desc,
		})
	}
	return infos
}

package tools

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"unicode/utf8"

	"charm.land/fantasy"
)

// ReadMDParams are the input parameters for the read_md tool.
type ReadMDParams struct {
	FilePath string `json:"file_path" description:"Absolute path to the markdown file to read"`
	Tree     bool   `json:"tree,omitempty" description:"Force tree view (heading structure + char counts)"`
	Section  string `json:"section,omitempty" description:"Section ID to extract (use tree view first to see IDs)"`
	Full     bool   `json:"full,omitempty" description:"Force full content even for large files"`
}

// NewReadMDTool creates a markdown-aware file reader.
// Small files (≤ treeThreshold chars) return full content by default.
// Large files return a heading tree. Agent can override with tree/full/section flags.
func NewReadMDTool(allowedPaths []string, treeThreshold int) fantasy.AgentTool {
	if treeThreshold <= 0 {
		treeThreshold = 5000
	}
	return fantasy.NewAgentTool(
		"read_md",
		schemaDescription(readMDDescription),
		func(ctx context.Context, params ReadMDParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if !isPathAllowed(params.FilePath, allowedPaths) {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: access denied: %q is not within an allowed directory", params.FilePath)), nil //nolint:lll
			}

			info, err := os.Stat(params.FilePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}
			if info.IsDir() {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %q is a directory, not a file", params.FilePath)), nil
			}

			source, err := os.ReadFile(params.FilePath)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
			}

			headings := parseHeadings(source)
			assignIDs(headings)

			// Section extraction mode.
			if params.Section != "" {
				section, err := extractSection(source, headings, params.Section)
				if err != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Error: %v", err)), nil
				}
				return fantasy.NewTextResponse(section), nil
			}

			// Tree mode (explicit or auto for large files).
			content := string(source)
			charCount := utf8.RuneCountInString(content)

			if params.Tree || (!params.Full && charCount > treeThreshold) {
				if len(headings) == 0 {
					// No headings — fall back to full content and warn.
					slog.Warn("read_md: no headings found, returning full content", "file", params.FilePath)
					return fantasy.NewTextResponse(truncateContent(content)), nil
				}
				return fantasy.NewTextResponse(renderTree(headings, source)), nil
			}

			// Full mode (explicit or auto for small files).
			return fantasy.NewTextResponse(truncateContent(content)), nil
		},
	)
}

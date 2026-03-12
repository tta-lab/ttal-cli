package tools

import (
	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// NewDefaultToolSet creates the standard tool set: bash, read_url, search_web.
// Optionally adds read, read_md, glob, grep when allowedPaths is non-empty.
// Panics if sbx or fetchBackend is nil.
func NewDefaultToolSet(sbx sandbox.Sandbox, fetchBackend ReadURLBackend, allowedPaths []string, treeThreshold int) []fantasy.AgentTool { //nolint:lll
	if sbx == nil {
		panic("agentloop/tools: NewDefaultToolSet: sbx must not be nil")
	}
	if fetchBackend == nil {
		panic("agentloop/tools: NewDefaultToolSet: fetchBackend must not be nil")
	}
	tools := []fantasy.AgentTool{
		NewBashTool(sbx),
		NewReadURLTool(fetchBackend, treeThreshold),
		NewSearchWebTool(nil), // uses its own transport with connection pooling
	}
	if len(allowedPaths) > 0 {
		tools = append(tools,
			NewReadTool(allowedPaths),
			NewReadMDTool(allowedPaths, treeThreshold),
			NewGlobTool(allowedPaths),
			NewGrepTool(allowedPaths),
		)
	}
	return tools
}

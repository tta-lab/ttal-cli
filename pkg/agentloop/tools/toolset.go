package tools

import (
	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// NewDefaultToolSet creates the standard tool set: bash, web_fetch, web_search.
// Panics if sbx or fetchBackend is nil.
func NewDefaultToolSet(sbx *sandbox.Sandbox, fetchBackend WebFetchBackend) []fantasy.AgentTool {
	if sbx == nil {
		panic("agentloop/tools: NewDefaultToolSet: sbx must not be nil")
	}
	if fetchBackend == nil {
		panic("agentloop/tools: NewDefaultToolSet: fetchBackend must not be nil")
	}
	return []fantasy.AgentTool{
		NewBashTool(sbx),
		NewWebFetchTool(fetchBackend),
		NewWebSearchTool(nil), // uses its own transport with connection pooling
	}
}

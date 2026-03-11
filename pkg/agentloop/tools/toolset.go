package tools

import (
	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// NewDefaultToolSet creates the standard tool set: bash, web_fetch, web_search.
func NewDefaultToolSet(sbx *sandbox.Sandbox, fetchBackend WebFetchBackend) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		NewBashTool(sbx),
		NewWebFetchTool(fetchBackend),
		NewWebSearchTool(nil), // uses its own transport with connection pooling
	}
}

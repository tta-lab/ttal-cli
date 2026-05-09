package doctor

import (
	"github.com/tta-lab/ttal-cli/internal/agentfs"
)

func countAgents(teamPath string) (int, error) {
	if teamPath == "" {
		return 0, nil
	}
	return agentfs.Count(teamPath)
}

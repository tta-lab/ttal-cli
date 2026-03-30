package review

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerEnvParts_AgentName(t *testing.T) {
	parts := buildReviewerEnvParts("abc12345-0000-0000-0000-000000000000", "pr-review-lead", runtime.ClaudeCode)
	var foundAgent, foundJobID bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=pr-review-lead" {
			foundAgent = true
		}
		if strings.HasPrefix(p, "TTAL_JOB_ID=") {
			foundJobID = true
		}
	}
	if !foundAgent {
		t.Errorf("TTAL_AGENT_NAME=pr-review-lead not found in env parts: %v", parts)
	}
	if !foundJobID {
		t.Errorf("TTAL_JOB_ID not found in env parts: %v", parts)
	}
}

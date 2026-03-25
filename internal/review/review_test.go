package review

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerEnvParts_AgentName(t *testing.T) {
	parts, err := buildReviewerEnvParts("pr-review-lead", runtime.ClaudeCode)
	if err != nil {
		t.Fatalf("buildReviewerEnvParts() error: %v", err)
	}
	var found bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=pr-review-lead" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_AGENT_NAME=pr-review-lead not found in env parts: %v", parts)
	}
}

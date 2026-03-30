package review

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestBuildReviewerEnvParts_AgentName(t *testing.T) {
	task := &taskwarrior.Task{UUID: "abc12345-0000-0000-0000-000000000000"}
	parts := buildReviewerEnvParts(task, "pr-review-lead", runtime.ClaudeCode)
	var foundAgent, foundJobID bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=pr-review-lead" {
			foundAgent = true
		}
		if p == "TTAL_JOB_ID=abc12345" {
			foundJobID = true
		}
	}
	if !foundAgent {
		t.Errorf("TTAL_AGENT_NAME=pr-review-lead not found in env parts: %v", parts)
	}
	if !foundJobID {
		t.Errorf("TTAL_JOB_ID=abc12345 not found in env parts: %v", parts)
	}
}

package planreview

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestBuildPlanReviewerEnvParts_ContainsJobID(t *testing.T) {
	task := &taskwarrior.Task{UUID: "f9a917aa-fc67-4aab-b398-18480e58ce86"}
	parts := buildPlanReviewerEnvParts(task, "plan-review-lead", runtime.ClaudeCode)
	var found bool
	for _, p := range parts {
		if p == "TTAL_JOB_ID=f9a917aa" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_JOB_ID=f9a917aa not found in env parts: %v", parts)
	}
}

func TestBuildPlanReviewerEnvParts_AgentNamePassthrough(t *testing.T) {
	task := &taskwarrior.Task{UUID: "abcd1234-0000-0000-0000-000000000000"}
	parts := buildPlanReviewerEnvParts(task, "custom-reviewer", runtime.ClaudeCode)
	var found bool
	for _, p := range parts {
		if p == "TTAL_AGENT_NAME=custom-reviewer" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_AGENT_NAME=custom-reviewer not found in env parts: %v", parts)
	}
}

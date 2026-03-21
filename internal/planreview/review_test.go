package planreview

import (
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildPlanReviewerEnvParts_ContainsJobID(t *testing.T) {
	uuid := "f9a917aa-fc67-4aab-b398-18480e58ce86"
	parts, err := buildPlanReviewerEnvParts(uuid, "plan-review-lead", runtime.ClaudeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

func TestBuildPlanReviewerEnvParts_IncludesTeamWhenSet(t *testing.T) {
	t.Setenv("TTAL_TEAM", "guion")
	uuid := "abcd1234-0000-0000-0000-000000000000"
	parts, err := buildPlanReviewerEnvParts(uuid, "plan-review-lead", runtime.ClaudeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var found bool
	for _, p := range parts {
		if p == "TTAL_TEAM=guion" {
			found = true
		}
	}
	if !found {
		t.Errorf("TTAL_TEAM=guion not found in env parts: %v", parts)
	}
}

func TestBuildPlanReviewerEnvParts_OmitsTeamWhenEmpty(t *testing.T) {
	_ = os.Unsetenv("TTAL_TEAM")
	uuid := "abcd1234-0000-0000-0000-000000000000"
	parts, err := buildPlanReviewerEnvParts(uuid, "plan-review-lead", runtime.ClaudeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range parts {
		if strings.HasPrefix(p, "TTAL_TEAM=") {
			t.Errorf("TTAL_TEAM should not be present when env var is empty, got: %s", p)
		}
	}
}

func TestBuildPlanReviewerEnvParts_ShortUUIDReturnsError(t *testing.T) {
	_, err := buildPlanReviewerEnvParts("short", "plan-review-lead", runtime.ClaudeCode)
	if err == nil {
		t.Error("expected error for UUID shorter than 8 chars")
	}
}

func TestBuildPlanReviewerEnvParts_AgentNamePassthrough(t *testing.T) {
	uuid := "abcd1234-0000-0000-0000-000000000000"
	parts, err := buildPlanReviewerEnvParts(uuid, "custom-reviewer", runtime.ClaudeCode)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
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

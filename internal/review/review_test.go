package review

import (
	"os"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestBuildReviewerEnvParts_AgentName(t *testing.T) {
	_ = os.Unsetenv("TTAL_TEAM")
	parts := buildReviewerEnvParts("pr-review-lead", runtime.ClaudeCode)
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

func TestBuildReviewerEnvParts_ForwardsTeam(t *testing.T) {
	t.Setenv("TTAL_TEAM", "guion")
	parts := buildReviewerEnvParts("pr-review-lead", runtime.ClaudeCode)
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

func TestBuildReviewerEnvParts_OmitsTeamWhenEmpty(t *testing.T) {
	_ = os.Unsetenv("TTAL_TEAM")
	parts := buildReviewerEnvParts("pr-review-lead", runtime.ClaudeCode)
	for _, p := range parts {
		if strings.HasPrefix(p, "TTAL_TEAM=") {
			t.Errorf("TTAL_TEAM should not be present when env var is empty, got: %s", p)
		}
	}
}

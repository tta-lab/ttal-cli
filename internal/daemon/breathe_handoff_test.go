package daemon

import (
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/route"
)

// TestBuildBreatheHandoff_AllCommandsFail verifies that when all breathe_context commands
// fail, the original req.Handoff is returned (not empty string).
func TestBuildBreatheHandoff_AllCommandsFail(t *testing.T) {
	cfg := &config.Config{}
	cfg.Prompts.BreatheContext = "false\nexit 1"

	req := BreatheRequest{Agent: "yuki", Handoff: "# Original handoff"}
	handoff, trigger := buildBreatheHandoff(cfg, req, "guion", nil)

	if handoff != "# Original handoff" {
		t.Errorf("expected original handoff on all-fail, got: %q", handoff)
	}
	if trigger != "" {
		t.Errorf("expected empty trigger, got: %q", trigger)
	}
}

// TestBuildBreatheHandoff_NoCommandsConfigured verifies the backward-compat path
// calls diaryReadToday (which falls back to req.Handoff when no diary entry exists).
func TestBuildBreatheHandoff_NoCommandsConfigured(t *testing.T) {
	cfg := &config.Config{}
	// BreatheContext is empty — no commands configured

	// Use an agent name that won't have a real diary entry
	req := BreatheRequest{Agent: "test-agent-no-diary-xyz", Handoff: "# Fallback handoff"}
	handoff, trigger := buildBreatheHandoff(cfg, req, "guion", nil)

	// With no diary entry for a nonexistent test agent, diaryReadToday returns req.Handoff
	if !strings.Contains(handoff, "# Fallback handoff") {
		t.Errorf("expected fallback handoff in result, got: %q", handoff)
	}
	if trigger != "" {
		t.Errorf("expected empty trigger, got: %q", trigger)
	}
}

// TestBuildBreatheHandoff_RouteComposition verifies that a routeReq appends role prompt,
// message, and returns the correct trigger.
func TestBuildBreatheHandoff_RouteComposition(t *testing.T) {
	cfg := &config.Config{}
	// Use a command that succeeds so we get deterministic context
	cfg.Prompts.BreatheContext = "echo context"

	req := BreatheRequest{Agent: "yuki", Handoff: "# Base"}
	routeReq := &route.Request{
		TaskUUID:   "abc12345",
		RolePrompt: "You are now a designer.",
		Message:    "Work on task abc12345.",
		Trigger:    "Work on task abc12345.",
		RoutedBy:   "astra",
	}

	handoff, trigger := buildBreatheHandoff(cfg, req, "guion", routeReq)

	if !strings.Contains(handoff, "## New Task Assignment") {
		t.Errorf("expected route role prompt section in handoff, got: %q", handoff)
	}
	if !strings.Contains(handoff, "You are now a designer.") {
		t.Errorf("expected role prompt text in handoff, got: %q", handoff)
	}
	if !strings.Contains(handoff, "Work on task abc12345.") {
		t.Errorf("expected route message in handoff, got: %q", handoff)
	}
	if trigger != "Work on task abc12345." {
		t.Errorf("expected trigger %q, got %q", "Work on task abc12345.", trigger)
	}
}

package daemon

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/route"
)

func TestHandleBreatheValidation(t *testing.T) {
	shellCfg := &config.Config{}

	tests := []struct {
		name    string
		req     BreatheRequest
		wantErr string
	}{
		{
			name:    "missing agent",
			req:     BreatheRequest{Agent: "", Handoff: "# Handoff"},
			wantErr: "missing agent name",
		},
		{
			name:    "empty handoff",
			req:     BreatheRequest{Agent: "kestrel", Handoff: ""},
			wantErr: "empty handoff prompt",
		},
		{
			name:    "session not found (valid agent + handoff but no tmux)",
			req:     BreatheRequest{Agent: "nonexistent-test-agent-xyz", Handoff: "# Handoff\n\nNext steps: continue"},
			wantErr: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := handleBreathe(shellCfg, nil, tt.req)
			if resp.OK {
				t.Fatalf("expected OK=false, got OK=true")
			}
			if tt.wantErr != "" && resp.Error == "" {
				t.Errorf("expected error containing %q, got empty", tt.wantErr)
			}
		})
	}
}

func TestHandleBreatheTeamDefault(t *testing.T) {
	shellCfg := &config.Config{}

	// team="" should default without panicking — it will fail at session check
	resp := handleBreathe(shellCfg, nil, BreatheRequest{
		Team:    "",
		Agent:   "nonexistent-test-agent-xyz",
		Handoff: "# Handoff",
	})
	// Should fail at session check, not at team validation
	if resp.OK {
		t.Fatalf("expected OK=false (no tmux session), got OK=true")
	}
}

// TestBuildCCRestartCmd verifies that --agent flag is present and correctly interpolated.
func TestBuildCCRestartCmd(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "")

	if !strings.Contains(cmd, "--resume session-abc") {
		t.Errorf("missing --resume flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--model sonnet") {
		t.Errorf("missing --model flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--agent kestrel") {
		t.Errorf("missing --agent flag: %q", cmd)
	}
	if !strings.Contains(cmd, "--dangerously-skip-permissions") {
		t.Errorf("missing --dangerously-skip-permissions flag: %q", cmd)
	}
	// Empty trigger should produce no -- separator
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdWithTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "New task: design auth. Run: ttal task get abc12345")
	if !strings.Contains(cmd, "-- 'New task:") {
		t.Errorf("missing trigger with -- separator: %q", cmd)
	}
}

func TestBuildCCRestartCmdEmptyTrigger(t *testing.T) {
	cmd := buildCCRestartCmd("session-123", "sonnet", "inke", "")
	if strings.Contains(cmd, "-- ") {
		t.Errorf("empty trigger should not produce -- separator: %q", cmd)
	}
}

// mockFrontend is a minimal frontend.Frontend for testing notification calls.
type mockFrontend struct {
	texts        []sentText
	textErr      error
	notifyCalled int
}

type sentText struct {
	agent string
	text  string
}

func (m *mockFrontend) SendText(_ context.Context, agent, text string) error {
	m.texts = append(m.texts, sentText{agent: agent, text: text})
	return m.textErr
}

// Implement remaining interface methods as no-ops.
func (m *mockFrontend) Start(_ context.Context) error { return nil }
func (m *mockFrontend) Stop(_ context.Context) error  { return nil }
func (m *mockFrontend) SendNotification(_ context.Context, _ string) error {
	m.notifyCalled++
	return nil
}
func (m *mockFrontend) SendVoice(_ context.Context, _ string, _ []byte) error   { return nil }
func (m *mockFrontend) SetReaction(_ context.Context, _ string, _ string) error { return nil }
func (m *mockFrontend) AskHuman(_ context.Context, _, _ string, _ []string) (string, bool, error) {
	return "", false, nil
}
func (m *mockFrontend) ClearTracking(_ context.Context, _ string) error { return nil }
func (m *mockFrontend) RegisterCommands(_ []frontend.Command) error     { return nil }
func (m *mockFrontend) AskHumanHTTPHandler() http.HandlerFunc           { return nil }

// TestSendBreatheNotification verifies that SendText is called with the agent's channel
// and the correct message, that a nil frontend is handled without panic, and that errors
// do not surface (they are logged only).
func TestSendBreatheNotification(t *testing.T) {
	t.Run("calls SendText with agent and correct message", func(t *testing.T) {
		m := &mockFrontend{}
		sendBreatheNotification(context.Background(), m, "kestrel", "default")
		if len(m.texts) != 1 {
			t.Fatalf("expected 1 SendText call, got %d", len(m.texts))
		}
		if m.texts[0].agent != "kestrel" {
			t.Errorf("expected agent %q, got %q", "kestrel", m.texts[0].agent)
		}
		if m.texts[0].text != "🫧 Deep breath. Fresh eyes." {
			t.Errorf("unexpected notification text: %q", m.texts[0].text)
		}
		if m.notifyCalled != 0 {
			t.Errorf("SendNotification should not be called, got %d calls", m.notifyCalled)
		}
	})

	t.Run("routes to correct agent channel for different agents", func(t *testing.T) {
		m := &mockFrontend{}
		sendBreatheNotification(context.Background(), m, "athena", "default")
		if len(m.texts) != 1 {
			t.Fatalf("expected 1 SendText call, got %d", len(m.texts))
		}
		if m.texts[0].agent != "athena" {
			t.Errorf("expected agent %q, got %q", "athena", m.texts[0].agent)
		}
	})

	t.Run("nil frontend does not panic", func(t *testing.T) {
		sendBreatheNotification(context.Background(), nil, "kestrel", "default")
	})

	t.Run("notification error does not propagate", func(t *testing.T) {
		m := &mockFrontend{textErr: fmt.Errorf("telegram down")}
		// Must not panic or return an error — errors are logged only.
		sendBreatheNotification(context.Background(), m, "kestrel", "default")
	})
}

func TestBuildCCRestartCmdApostropheEscaping(t *testing.T) {
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel", "it's a test")
	if !strings.Contains(cmd, "it'\\''s a test") {
		t.Errorf("apostrophe not escaped correctly: %q", cmd)
	}
}

// TestComposeHandoff covers all 4 branches of composeHandoff:
// no file, role prompt only, message only, and both.
func TestComposeHandoff(t *testing.T) {
	base := "# Base Handoff\n\nContext here."

	t.Run("no routing file returns base handoff unchanged", func(t *testing.T) {
		handoff, trigger, err := composeHandoff("test-composehandoff-no-route-xyz", base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if handoff != base {
			t.Errorf("expected base handoff unchanged, got %q", handoff)
		}
		if trigger != "" {
			t.Errorf("expected empty trigger, got %q", trigger)
		}
	})

	t.Run("role prompt only appended with section header", func(t *testing.T) {
		agent := "test-composehandoff-roleprompt-xyz"
		if err := route.Stage(agent, route.Request{
			TaskUUID:   "task-abc",
			RolePrompt: "Build the auth module.",
			Trigger:    "auth task ready",
		}); err != nil {
			t.Fatalf("stage failed: %v", err)
		}
		handoff, trigger, err := composeHandoff(agent, base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(handoff, "## New Task Assignment") {
			t.Errorf("expected section header in handoff: %q", handoff)
		}
		if !strings.Contains(handoff, "Build the auth module.") {
			t.Errorf("expected role prompt in handoff: %q", handoff)
		}
		if trigger != "auth task ready" {
			t.Errorf("expected trigger %q, got %q", "auth task ready", trigger)
		}
	})

	t.Run("message only appended without section header", func(t *testing.T) {
		agent := "test-composehandoff-message-xyz"
		if err := route.Stage(agent, route.Request{
			TaskUUID: "task-def",
			Message:  "Extra context for you.",
			Trigger:  "msg trigger",
		}); err != nil {
			t.Fatalf("stage failed: %v", err)
		}
		handoff, trigger, err := composeHandoff(agent, base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(handoff, "## New Task Assignment") {
			t.Errorf("should not have section header when no role prompt: %q", handoff)
		}
		if !strings.Contains(handoff, "Extra context for you.") {
			t.Errorf("expected message in handoff: %q", handoff)
		}
		if trigger != "msg trigger" {
			t.Errorf("expected trigger %q, got %q", "msg trigger", trigger)
		}
	})

	t.Run("both role prompt and message appended", func(t *testing.T) {
		agent := "test-composehandoff-both-xyz"
		if err := route.Stage(agent, route.Request{
			TaskUUID:   "task-ghi",
			RolePrompt: "Design the API.",
			Message:    "See ticket #42.",
			Trigger:    "design task",
		}); err != nil {
			t.Fatalf("stage failed: %v", err)
		}
		handoff, trigger, err := composeHandoff(agent, base)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(handoff, "## New Task Assignment") {
			t.Errorf("expected section header: %q", handoff)
		}
		if !strings.Contains(handoff, "Design the API.") {
			t.Errorf("expected role prompt: %q", handoff)
		}
		if !strings.Contains(handoff, "See ticket #42.") {
			t.Errorf("expected message: %q", handoff)
		}
		if trigger != "design task" {
			t.Errorf("expected trigger %q, got %q", "design task", trigger)
		}
	})
}

// TestBuildCCRestartCmdAgentInterpolation verifies agent name is not swapped with session/model.
func TestBuildCCRestartCmdAgentInterpolation(t *testing.T) {
	cmd := buildCCRestartCmd("my-session", "opus", "athena", "")
	if !strings.Contains(cmd, "--agent athena") {
		t.Errorf("agent name not correctly interpolated, got: %q", cmd)
	}
	// Ensure model is not placed in the agent slot
	if strings.Contains(cmd, "--agent opus") {
		t.Errorf("model leaked into --agent slot: %q", cmd)
	}
}

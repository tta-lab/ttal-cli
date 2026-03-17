package daemon

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
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
	cmd := buildCCRestartCmd("session-abc", "sonnet", "kestrel")

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
}

// mockFrontend is a minimal frontend.Frontend for testing notification calls.
type mockFrontend struct {
	texts   []sentText
	textErr error
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
func (m *mockFrontend) Start(_ context.Context) error                           { return nil }
func (m *mockFrontend) Stop(_ context.Context) error                            { return nil }
func (m *mockFrontend) SendNotification(_ context.Context, _ string) error      { return nil }
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

// TestBuildCCRestartCmdAgentInterpolation verifies agent name is not swapped with session/model.
func TestBuildCCRestartCmdAgentInterpolation(t *testing.T) {
	cmd := buildCCRestartCmd("my-session", "opus", "athena")
	if !strings.Contains(cmd, "--agent athena") {
		t.Errorf("agent name not correctly interpolated, got: %q", cmd)
	}
	// Ensure model is not placed in the agent slot
	if strings.Contains(cmd, "--agent opus") {
		t.Errorf("model leaked into --agent slot: %q", cmd)
	}
}

package daemon

import (
	"context"
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/sendfmt"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// ReplyHint is a thin wrapper around sendfmt.ReplyHint, kept for back-compat
// with cmd/send.go which composes its own alert body and appends the canonical
// hint. New code should call sendfmt.ReplyHint directly.
func ReplyHint(senderAddr string) string {
	return sendfmt.ReplyHint(senderAddr)
}

// formatAgentMessage formats an agent-to-agent message for delivery.
// Delegates to sendfmt.Format with channel="agent" so the layout matches
// telegram/matrix inbound formatters (single-line header).
func formatAgentMessage(fromAgent, text string) string {
	return sendfmt.Format(sendfmt.Envelope{
		Channel:    "agent",
		SenderName: fromAgent,
		Body:       text,
		ReplyAlias: fromAgent,
	})
}

// deliverToAgent sends text to an agent via its runtime adapter.
// Falls back to tmux for local tmux-backed runtimes, frontend notification for others.
func deliverToAgent(
	registry *adapterRegistry, cfg *config.Config,
	frontends map[string]frontend.Frontend,
	agentName, text string,
) error {
	if registry != nil {
		if adapter, ok := registry.get("default", agentName); ok {
			return adapter.SendMessage(context.Background(), text)
		}
	}
	// Fallback: tmux for local runtimes, frontend notification for others.
	rt := cfg.RuntimeForAgent(agentName)
	if rt.IsTmuxBacked() {
		session := config.AgentSessionName(agentName)
		return tmux.SendKeys(session, agentName, text)
	}
	if rt == runtime.Codex {
		return fmt.Errorf("codex adapter not available for %s (start failed?)", agentName)
	}
	// Non-CC agent with no adapter — send via frontend notification
	fe, ok := frontends["default"]
	if !ok {
		return fmt.Errorf("no frontend for team default")
	}
	msg := fmt.Sprintf("[undelivered → %s] %s", agentName, text)
	return fe.SendNotification(context.Background(), msg)
}

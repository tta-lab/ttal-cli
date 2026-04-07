package daemon

import (
	"context"
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// ReplyHint returns a footer line instructing the receiver how to reply to senderAddr.
func ReplyHint(senderAddr string) string {
	return fmt.Sprintf("Reply with: ttal send --to %s \"your message\"", senderAddr)
}

// formatAgentMessage formats an agent-to-agent message for delivery.
// Includes a reply hint footer so the receiver knows how to respond.
func formatAgentMessage(fromAgent, text string) string {
	return fmt.Sprintf("[agent from:%s]\n%s\n\n---\n%s", fromAgent, text, ReplyHint(fromAgent))
}

// deliverToAgent sends text to an agent via its runtime adapter.
// Falls back to tmux for CC agents, frontend notification for others.
func deliverToAgent(
	registry *adapterRegistry, mcfg *config.DaemonConfig,
	frontends map[string]frontend.Frontend,
	agentName, text string,
) error {
	if registry != nil {
		if adapter, ok := registry.get(config.DefaultTeamName, agentName); ok {
			return adapter.SendMessage(context.Background(), text)
		}
	}
	// Fallback: tmux for CC agents, frontend notification for others
	rt := mcfg.RuntimeForAgent(config.DefaultTeamName, "", agentName)
	if rt == runtime.ClaudeCode {
		session := config.AgentSessionName(agentName)
		return tmux.SendKeys(session, agentName, text)
	}
	if rt == runtime.Codex {
		return fmt.Errorf("codex adapter not available for %s (start failed?)", agentName)
	}
	// Non-CC agent with no adapter — send via frontend notification
	fe, ok := frontends[config.DefaultTeamName]
	if !ok {
		return fmt.Errorf("no frontend for team default")
	}
	msg := fmt.Sprintf("[undelivered → %s] %s", agentName, text)
	return fe.SendNotification(context.Background(), msg)
}

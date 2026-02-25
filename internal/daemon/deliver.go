package daemon

import (
	"context"
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// formatInboundMessage formats a Telegram message for delivery to CC.
func formatInboundMessage(_, senderName, text string) string {
	return fmt.Sprintf("[telegram from:%s] %s", senderName, text)
}

// formatAgentMessage formats an agent-to-agent message for delivery.
func formatAgentMessage(fromAgent, text string) string {
	return fmt.Sprintf("[agent from:%s]\n%s", fromAgent, text)
}

// deliverToAgent sends text to an agent via its runtime adapter.
// Falls back to direct tmux send-keys if no adapter is registered.
func deliverToAgent(registry *adapterRegistry, agentName, text string) error {
	if registry != nil {
		if adapter, ok := registry.get(agentName); ok {
			return adapter.SendMessage(context.Background(), text)
		}
	}
	// Fallback: direct tmux for agents not yet using adapters
	session := config.AgentSessionName(agentName)
	return tmux.SendKeys(session, agentName, text)
}

package daemon

import (
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

// deliverToAgent sends text to an agent's tmux session via send-keys + Enter.
// Session name = "session-<agentName>" (convention). Window name = agent name.
func deliverToAgent(agentName, text string) error {
	session := config.AgentSessionName(agentName)
	return tmux.SendKeys(session, agentName, text)
}

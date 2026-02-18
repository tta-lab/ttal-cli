package daemon

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// formatInboundMessage formats a Telegram message for delivery to CC in zellij.
func formatInboundMessage(_, senderName, text string) string {
	return fmt.Sprintf("[telegram from:%s] %s", senderName, text)
}

// formatAgentMessage formats an agent-to-agent message for delivery via zellij.
func formatAgentMessage(fromAgent, text string) string {
	return fmt.Sprintf("[agent from:%s]\n%s", fromAgent, text)
}

// deliverToZellij sends text to an agent's zellij tab via write-chars + Enter.
// Tab name = agent name (convention).
func deliverToZellij(session, agentName, text string) error {
	return zellij.WriteChars(session, agentName, "", text)
}

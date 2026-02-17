package daemon

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// formatInboundMessage formats a Telegram message for delivery to CC in zellij.
func formatInboundMessage(agentName, senderName, text string) string {
	return fmt.Sprintf("[telegram from:%s]\n%s\n\nTo reply: ttal send --from %s \"your reply\"",
		senderName, text, agentName)
}

// formatAgentMessage formats an agent-to-agent message for delivery via zellij.
func formatAgentMessage(fromAgent, text string) string {
	return fmt.Sprintf("[agent from:%s]\n%s", fromAgent, text)
}

// deliverToZellij sends text to a zellij pane via write-chars + Enter.
// Text is sanitized to prevent shell injection before delivery.
func deliverToZellij(cfg ZellijConfig, text string) error {
	if cfg.Session == "" {
		return fmt.Errorf("zellij config missing session")
	}
	return zellij.WriteChars(cfg.Session, cfg.Tab, "", text)
}

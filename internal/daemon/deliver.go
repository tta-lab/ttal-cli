package daemon

import (
	"context"
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
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
// K8s teams use kubectl exec into the team pod. Falls back to tmux for CC agents, notification bot for others.
func deliverToAgent(registry *adapterRegistry, mcfg *config.DaemonConfig, teamName, agentName, text string) error {
	// K8s delivery — reuse team pod from startup
	if pod, ok := k8sPods[teamName]; ok {
		return pod.SendKeys(agentName, text)
	}

	if registry != nil {
		if adapter, ok := registry.get(teamName, agentName); ok {
			return adapter.SendMessage(context.Background(), text)
		}
	}
	// Fallback: tmux for CC agents, notification bot for others
	rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
	if rt == runtime.ClaudeCode {
		session := config.AgentSessionName(teamName, agentName)
		return tmux.SendKeys(session, agentName, text)
	}
	// Non-CC agent with no adapter — send via notification bot
	team, ok := mcfg.Teams[teamName]
	if !ok {
		return fmt.Errorf("no team config for %s", teamName)
	}
	msg := fmt.Sprintf("[undelivered → %s] %s", agentName, text)
	return notify.SendWithConfig(team.NotificationToken, team.ChatID, msg)
}

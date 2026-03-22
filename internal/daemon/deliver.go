package daemon

import (
	"context"
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// formatAgentMessage formats an agent-to-agent message for delivery.
func formatAgentMessage(fromAgent, text string) string {
	return fmt.Sprintf("[agent from:%s]\n%s", fromAgent, text)
}

// deliverToAgent sends text to an agent via its runtime adapter.
// Falls back to tmux for CC agents, frontend notification for others.
func deliverToAgent(
	registry *adapterRegistry, mcfg *config.DaemonConfig,
	frontends map[string]frontend.Frontend,
	teamName, agentName, text string,
) error {
	if registry != nil {
		if adapter, ok := registry.get(teamName, agentName); ok {
			return adapter.SendMessage(context.Background(), text)
		}
	}
	// Fallback: tmux for CC agents, frontend notification for others
	rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
	if rt == runtime.ClaudeCode {
		// Prefer task-scoped session if one exists.
		if tsSession := tmux.FindSessionByPrefix("ts-", "-"+agentName); tsSession != "" {
			return tmux.SendKeys(tsSession, agentName, text)
		}
		// Fall back to persistent session.
		session := config.AgentSessionName(teamName, agentName)
		return tmux.SendKeys(session, agentName, text)
	}
	// Non-CC agent with no adapter — send via frontend notification
	fe, ok := frontends[teamName]
	if !ok {
		return fmt.Errorf("no frontend for team %s", teamName)
	}
	msg := fmt.Sprintf("[undelivered → %s] %s", agentName, text)
	return fe.SendNotification(context.Background(), msg)
}

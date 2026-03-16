package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// workerWindow is the tmux window name used by all worker sessions.
const workerWindow = "worker"

// persistMsg persists a message and logs a warning if it fails.
func persistMsg(msgSvc *message.Service, p message.CreateParams) {
	if _, err := msgSvc.Create(context.Background(), p); err != nil {
		log.Printf("[daemon] message persist failed (sender=%s): %v", p.Sender, err)
	}
}

// handleSend routes an incoming SendRequest based on From/To fields.
// Resolves team from agent name or the Team field in the request.
func handleSend(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(mcfg, frontends, msgSvc, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgent(mcfg, registry, frontends, msgSvc, req)
	case req.From != "":
		return handleFrom(mcfg, frontends, msgSvc, req)
	case req.To != "":
		return handleTo(mcfg, registry, frontends, msgSvc, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleFrom sends a message from an agent to the human via the team's frontend.
func handleFrom(
	mcfg *config.DaemonConfig,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	ta := resolveAgent(mcfg, req.Team, req.From)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	fe, ok := frontends[ta.TeamName]
	if !ok {
		return fmt.Errorf("no frontend configured for team %s (agent %s)", ta.TeamName, req.From)
	}
	rt := mcfg.AgentRuntimeForTeam(ta.TeamName, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: mcfg.Global.UserName(), Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	return fe.SendText(context.Background(), ta.AgentName, req.Message)
}

// handleTo delivers a message to an agent via its runtime adapter.
// Falls back to worker session delivery when the recipient is a hex UUID.
// Human→worker messages are sent as bare text (no [agent from:] prefix).
func handleTo(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		session, err := resolveWorker(req.To)
		if err != nil {
			return fmt.Errorf("unknown agent or worker %s: %w", req.To, err)
		}
		log.Printf("[daemon] human-to-worker: %s → %s (%s)", mcfg.Global.UserName(), req.To, session)
		return dispatchToWorker(msgSvc, session, message.CreateParams{
			Sender: mcfg.Global.UserName(), Recipient: "worker:" + req.To,
			Content: req.Message, Team: req.Team, Channel: message.ChannelCLI,
		}, req.Message)
	}
	persistMsg(msgSvc, message.CreateParams{
		Sender: mcfg.Global.UserName(), Recipient: req.To, Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI,
	})
	return deliverToAgent(registry, mcfg, frontends, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another.
// Falls back to worker session delivery when the recipient is a hex UUID.
func handleAgentToAgent(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	fromTA := resolveAgent(mcfg, req.Team, req.From)
	if fromTA == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	toTA := resolveAgent(mcfg, req.Team, req.To)
	msg := formatAgentMessage(req.From, req.Message)
	if toTA == nil {
		session, err := resolveWorker(req.To)
		if err != nil {
			return fmt.Errorf("unknown agent or worker %s: %w", req.To, err)
		}
		rt := mcfg.AgentRuntimeForTeam(fromTA.TeamName, req.From)
		log.Printf("[daemon] agent-to-worker: %s → %s (%s)", req.From, req.To, session)
		return dispatchToWorker(msgSvc, session, message.CreateParams{
			Sender: req.From, Recipient: "worker:" + req.To, Content: req.Message,
			Team: fromTA.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
		}, msg)
	}
	rt := mcfg.AgentRuntimeForTeam(fromTA.TeamName, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: req.To, Content: req.Message,
		Team: fromTA.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
	return deliverToAgent(registry, mcfg, frontends, toTA.TeamName, req.To, msg)
}

// resolveAgent finds an agent by name, using team hint if provided.
func resolveAgent(mcfg *config.DaemonConfig, teamHint, agentName string) *config.TeamAgent {
	if teamHint != "" {
		ta, ok := mcfg.FindAgentInTeam(teamHint, agentName)
		if ok {
			return ta
		}
	}
	ta, ok := mcfg.FindAgent(agentName)
	if ok {
		return ta
	}
	return nil
}

// resolveWorker finds a tmux session for a worker identified by hex UUID prefix.
// Session names follow the format: w-{uuid[:8]}-{slug}.
// idPrefix must be at least 8 hex characters (case-insensitive).
func resolveWorker(idPrefix string) (string, error) {
	normalized := strings.ToLower(idPrefix)
	if len(normalized) < 8 {
		return "", fmt.Errorf("not a worker UUID: %q", idPrefix)
	}
	for _, c := range normalized {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("not a worker UUID: %q", idPrefix)
		}
	}
	sessions, err := tmux.ListSessions()
	if err != nil {
		return "", fmt.Errorf("list tmux sessions: %w", err)
	}
	if sessions == nil {
		return "", fmt.Errorf("no tmux server running")
	}
	prefix := "w-" + normalized[:8]
	for _, s := range sessions {
		if strings.HasPrefix(s, prefix) {
			return s, nil
		}
	}
	return "", fmt.Errorf("no worker session for %s", idPrefix)
}

// dispatchToWorker persists a message and delivers it to a worker tmux session.
func dispatchToWorker(msgSvc *message.Service, session string, params message.CreateParams, text string) error {
	persistMsg(msgSvc, params)
	return deliverToWorker(session, text)
}

// deliverToWorker sends a message to a worker's tmux session.
func deliverToWorker(session, text string) error {
	return tmux.SendKeys(session, workerWindow, text)
}

// handleStatusUpdate writes agent context status to the status directory.
func handleStatusUpdate(req StatusUpdateRequest) {
	team := req.Team
	if team == "" {
		team = config.DefaultTeamName
	}
	s := status.AgentStatus{
		Agent:               req.Agent,
		ContextUsedPct:      req.ContextUsedPct,
		ContextRemainingPct: req.ContextRemainingPct,
		ModelID:             req.ModelID,
		SessionID:           req.SessionID,
		UpdatedAt:           time.Now(),
	}
	if err := status.WriteAgent(team, s); err != nil {
		log.Printf("[daemon] failed to write status for %s/%s: %v", team, req.Agent, err)
	}
}

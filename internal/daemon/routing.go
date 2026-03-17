package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
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

// breatheAgentModel holds the model info resolved from the agent's status file.
type breatheAgentModel struct {
	model, ccVersion, modelName string
}

// resolveAgentModel reads the current model/CC version from the agent's status file.
func resolveAgentModel(team, agent string) breatheAgentModel {
	s, _ := status.ReadAgent(team, agent)
	info := breatheAgentModel{model: "sonnet"}
	if s != nil {
		if s.ModelID != "" {
			info.model = s.ModelID
		}
		info.ccVersion = s.CCVersion
		info.modelName = s.ModelName
	}
	return info
}

// injectSecretsToSession loads .env and injects each key into the tmux session environment.
func injectSecretsToSession(sessionName string) {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		log.Printf("[breathe] warning: .env load failed, secrets may be missing: %v", err)
	}
	for k, v := range dotEnv {
		if err := tmux.SetEnv(sessionName, k, v); err != nil {
			log.Printf("[breathe] warning: failed to inject %s into session %s: %v", k, sessionName, err)
		}
	}
}

// buildCCRestartCmd returns the claude --resume command for a breathe restart.
// Extracted for unit testing.
func buildCCRestartCmd(sessionID, model, agent string) string {
	return fmt.Sprintf(
		"claude --resume %s --model %s --dangerously-skip-permissions --agent %s",
		sessionID, model, agent,
	)
}

// handleBreathe restarts an agent's CC session with a handoff prompt.
// shellCfg is loaded once at daemon startup and passed in — never loaded per-request.
// frontends is the full team→frontend map; team resolution happens inside.
func handleBreathe(shellCfg *config.Config, frontends map[string]frontend.Frontend, req BreatheRequest) SendResponse {
	team := req.Team
	if team == "" {
		team = config.DefaultTeamName
	}
	if req.Agent == "" {
		return SendResponse{OK: false, Error: "missing agent name"}
	}
	if req.Handoff == "" {
		return SendResponse{OK: false, Error: "empty handoff prompt"}
	}

	sessionName := config.AgentSessionName(team, req.Agent)
	windowName := req.Agent

	// 1. Verify session exists BEFORE doing anything
	if !tmux.SessionExists(sessionName) {
		return SendResponse{OK: false, Error: fmt.Sprintf("session %q not found", sessionName)}
	}

	// 2. Get agent's CWD from tmux pane
	cwd, err := tmux.GetPaneCwd(sessionName, windowName)
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("get cwd: %v", err)}
	}
	if cwd == "" {
		return SendResponse{OK: false, Error: "pane CWD is empty — pane may have exited"}
	}

	// 3. Get model, CC version, and git branch
	am := resolveAgentModel(team, req.Agent)
	gitBranch := gitutil.BranchName(cwd)
	if gitBranch == "" {
		log.Printf("[breathe] %s: could not detect git branch for %s — leaving empty", req.Agent, cwd)
	}

	// 4. Write synthetic JSONL session (BEFORE killing anything)
	projectDir, err := breathe.CCProjectDir(cwd)
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("resolve CC project dir: %v", err)}
	}
	newSessionID, err := breathe.WriteSyntheticSession(projectDir, breathe.SessionConfig{
		CWD:       cwd,
		CCVersion: am.ccVersion,
		GitBranch: gitBranch,
		Handoff:   req.Handoff,
	})
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("write session: %v", err)}
	}

	// 5. Update status file with new session ID
	if err := status.WriteAgent(team, status.AgentStatus{
		Agent:               req.Agent,
		SessionID:           newSessionID,
		ContextUsedPct:      0,
		ContextRemainingPct: 100,
		ModelID:             am.model,
		ModelName:           am.modelName,
		CCVersion:           am.ccVersion,
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		log.Printf("[breathe] warning: failed to write status for %s/%s: %v", team, req.Agent, err)
	}

	// 6. Build restart command
	ccCmd := buildCCRestartCmd(newSessionID, am.model, req.Agent)
	fullCmd := shellCfg.BuildEnvShellCommand([]string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", req.Agent),
		fmt.Sprintf("TTAL_TEAM=%s", team),
	}, ccCmd)

	// 7. Inject .env secrets and respawn
	injectSecretsToSession(sessionName)
	log.Printf("[breathe] %s: restarting with session %s (model: %s)", req.Agent, newSessionID, am.model)
	if err := tmux.RespawnWindow(sessionName, windowName, cwd, fullCmd); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("respawn: %v", err)}
	}

	log.Printf("[breathe] %s: fresh breath taken", req.Agent)

	// 8. Notify via frontend
	sendBreatheNotification(context.Background(), frontends[team], req.Agent, team)

	return SendResponse{OK: true}
}

// sendBreatheNotification sends the post-breathe notification through the agent's own channel.
// Extracted for unit testing. A nil frontend is valid — logs and skips.
func sendBreatheNotification(ctx context.Context, fe frontend.Frontend, agent, team string) {
	if fe == nil {
		log.Printf("[breathe] %s: no frontend for team %q — notification skipped", agent, team)
		return
	}
	if err := fe.SendText(ctx, agent, "🫧 Deep breath. Fresh eyes."); err != nil {
		log.Printf("[breathe] warning: failed to send notification: %v", err)
	}
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

package daemon

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/breathe"
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

// handleBreathe restarts an agent's CC session with a handoff prompt.
// shellCfg is loaded once at daemon startup and passed in — never loaded per-request.
func handleBreathe(_ *config.DaemonConfig, shellCfg *config.Config, req BreatheRequest) SendResponse {
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

	// 3. Get model and CC version from status file
	agentStatus, _ := status.ReadAgent(team, req.Agent)
	model := "sonnet"
	ccVersion := ""
	if agentStatus != nil {
		if agentStatus.ModelID != "" {
			model = agentStatus.ModelID
		}
		ccVersion = agentStatus.CCVersion
	}

	// 4. Get git branch
	gitBranch := gitBranchForDir(cwd)

	// 5. Write synthetic JSONL session (BEFORE killing anything)
	projectDir := breathe.CCProjectDir(cwd)
	cfg := breathe.SessionConfig{
		CWD:       cwd,
		CCVersion: ccVersion,
		GitBranch: gitBranch,
		Handoff:   req.Handoff,
	}
	newSessionID, err := breathe.WriteSyntheticSession(projectDir, cfg)
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("write session: %v", err)}
	}

	// 6. Update status file with new session ID
	newStatus := status.AgentStatus{
		Agent:               req.Agent,
		SessionID:           newSessionID,
		ContextUsedPct:      0,
		ContextRemainingPct: 100,
		ModelID:             model,
		CCVersion:           ccVersion,
		UpdatedAt:           time.Now().UTC(),
	}
	if agentStatus != nil {
		newStatus.ModelName = agentStatus.ModelName
	}
	_ = status.WriteAgent(team, newStatus)

	// 7. Build restart command
	envParts := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", req.Agent),
		fmt.Sprintf("TTAL_TEAM=%s", team),
	}
	ccCmd := fmt.Sprintf("claude --resume %s --model %s --dangerously-skip-permissions", newSessionID, model)
	fullCmd := shellCfg.BuildEnvShellCommand(envParts, ccCmd)

	// 8. Inject .env secrets into tmux session
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		log.Printf("[breathe] warning: .env load failed, secrets may be missing: %v", err)
	}
	for k, v := range dotEnv {
		_ = tmux.SetEnv(sessionName, k, v)
	}

	// 9. Respawn window (kills existing CC process + starts new one)
	log.Printf("[breathe] %s: restarting with session %s (model: %s)", req.Agent, newSessionID, model)
	if err := tmux.RespawnWindow(sessionName, windowName, cwd, fullCmd); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("respawn: %v", err)}
	}

	log.Printf("[breathe] %s: fresh breath taken ✓", req.Agent)
	return SendResponse{OK: true}
}

// gitBranchForDir returns the current git branch for the given directory, or "main" if unknown.
func gitBranchForDir(cwd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", cwd, "symbolic-ref", "--short", "HEAD")
	cmd.Stderr = nil
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(out))
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

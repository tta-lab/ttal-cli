package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/breathe"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/route"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// workerWindow is the tmux window name used by all worker sessions.
const workerWindow = worker.CoderAgentName

// persistMsg persists a message and logs a warning if it fails.
// msgSvc may be nil in tests — the call is a no-op in that case.
func persistMsg(msgSvc *message.Service, p message.CreateParams) {
	if msgSvc == nil {
		return
	}
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
	// Ordering matters: "system" check must follow To=="human" (system never sends to human)
	// and precede the generic From+To agent-to-agent case (system is not a named agent).
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(mcfg, frontends, msgSvc, req)
	case req.From == "system" && req.To != "":
		return handleSystemToAgent(mcfg, registry, frontends, msgSvc, req)
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
	// Human-originated: no runtime attribution (humans don't have a runtime entry).
	persistMsg(msgSvc, message.CreateParams{
		Sender: mcfg.Global.UserName(), Recipient: req.To, Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI,
	})
	return deliverToAgent(registry, mcfg, frontends, ta.TeamName, req.To, req.Message)
}

// handleSystemToAgent delivers a system-originated message to an agent as bare text.
// No [agent from:] prefix is added — used for automated triggers like /breathe
// where CC must receive raw text to recognize it as a skill trigger.
func handleSystemToAgent(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	rt := mcfg.AgentRuntimeForTeam(ta.TeamName, req.To)
	persistMsg(msgSvc, message.CreateParams{
		Sender: "system", Recipient: req.To, Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	return deliverToAgent(registry, mcfg, frontends, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another.
// Falls back to worker session delivery when the recipient is a hex UUID.
// The sender may also be a worker hex UUID (e.g. from ttal alert in a worker session).
func handleAgentToAgent(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	fromTA := resolveAgent(mcfg, req.Team, req.From)
	if fromTA == nil {
		if _, err := resolveWorker(req.From); err != nil {
			// session string discarded — only validating that the hex ID resolves
			return fmt.Errorf("unknown agent or worker: %s", req.From)
		}
	}
	senderTeam := req.Team
	if senderTeam == "" {
		senderTeam = config.DefaultTeamName
	}
	if fromTA != nil {
		senderTeam = fromTA.TeamName
	} else if senderTeam == config.DefaultTeamName && req.Team == "" {
		log.Printf("[daemon] worker sender %s: team unknown, attributing to default team", req.From)
	}

	toTA := resolveAgent(mcfg, req.Team, req.To)
	msg := formatAgentMessage(req.From, req.Message)
	if toTA == nil {
		session, err := resolveWorker(req.To)
		if err != nil {
			return fmt.Errorf("unknown agent or worker %s: %w", req.To, err)
		}
		rt := mcfg.AgentRuntimeForTeam(senderTeam, req.From)
		log.Printf("[daemon] agent-to-worker: %s → %s (%s)", req.From, req.To, session)
		return dispatchToWorker(msgSvc, session, message.CreateParams{
			Sender: req.From, Recipient: "worker:" + req.To, Content: req.Message,
			Team: senderTeam, Channel: message.ChannelCLI, Runtime: &rt,
		}, msg)
	}
	rt := mcfg.AgentRuntimeForTeam(senderTeam, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: req.To, Content: req.Message,
		Team: senderTeam, Channel: message.ChannelCLI, Runtime: &rt,
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
	s, err := status.ReadAgent(team, agent)
	if err != nil {
		log.Printf("[breathe] %s: warning: could not read status file, using default model: %v", agent, err)
	}
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

// injectSecretsToSession loads .env and injects allowlisted vars into the tmux session environment.
// Secrets (tokens) are blocked — authenticated operations go through the daemon.
func injectSecretsToSession(sessionName string) {
	dotEnv, err := config.LoadDotEnv()
	if err != nil {
		log.Printf("[breathe] warning: .env load failed, secrets may be missing: %v", err)
	}
	for k, v := range dotEnv {
		if !env.IsAllowedForSession(k) {
			continue
		}
		if err := tmux.SetEnv(sessionName, k, v); err != nil {
			log.Printf("[breathe] warning: failed to inject %s into session %s: %v", k, sessionName, err)
		}
	}
}

// buildCCRestartCmd returns the claude --resume command for a breathe restart.
// trigger, if non-empty, is appended as a positional arg after --.
// When trigger is empty (self-breathe), no -- separator is added.
// Extracted for unit testing.
func buildCCRestartCmd(sessionID, model, agent, trigger string) string {
	cmd := fmt.Sprintf(
		"claude --resume %s --model %s --dangerously-skip-permissions --agent %s",
		sessionID, model, agent,
	)
	if trigger != "" {
		escaped := strings.ReplaceAll(trigger, "'", "'\\''")
		cmd += fmt.Sprintf(" -- '%s'", escaped)
	}
	return cmd
}

// resolveBrCWD returns the agent's working directory for a breathe restart.
// It prefers the live pane CWD; if the session is dead or pane CWD is unavailable,
// it falls back to the registered agent workspace path from config.
// Returns sessionAlive so the caller can skip KillSession on a dead session.
func resolveBrCWD(sessionName, windowName, agent string, cfg *config.Config) (string, bool, error) {
	var cwd string
	sessionAlive := tmux.SessionExists(sessionName)
	if sessionAlive {
		var err error
		cwd, err = tmux.GetPaneCwd(sessionName, windowName)
		if err != nil {
			log.Printf("[breathe] %s: pane CWD unavailable (%v), falling back to agent path", agent, err)
		} else if cwd == "" {
			log.Printf("[breathe] %s: live session returned empty pane CWD, falling back to agent path", agent)
		}
	}
	if cwd == "" {
		cwd = cfg.AgentPath(agent)
		if cwd == "" {
			return "", sessionAlive, fmt.Errorf("cannot resolve agent workspace path — team path not configured")
		}
		log.Printf("[breathe] %s: using registered agent path as CWD: %s", agent, cwd)
	}
	return cwd, sessionAlive, nil
}

// breatheSessionPlan holds the resolved session names and CWD for a breathe operation.
type breatheSessionPlan struct {
	oldSessionName string
	newSessionName string
	windowName     string
	cwd            string
}

// resolveBreatheSessions determines old/new session names and CWD for a persistent-agent breathe.
func resolveBreatheSessions(
	req BreatheRequest, team string, shellCfg *config.Config,
) (breatheSessionPlan, error) {
	persistName := config.AgentSessionName(team, req.Agent)
	// TODO(fork-model): session override hook for future fork-model graduation
	cwdSession := persistName
	if req.SessionName != "" {
		cwdSession = req.SessionName
	}
	cwd, _, err := resolveBrCWD(cwdSession, req.Agent, req.Agent, shellCfg)
	if err != nil {
		return breatheSessionPlan{}, err
	}
	oldName := persistName
	if req.SessionName != "" {
		oldName = req.SessionName
	}
	return breatheSessionPlan{
		oldSessionName: oldName,
		newSessionName: persistName,
		windowName:     req.Agent,
		cwd:            cwd,
	}, nil
}

// selectBreatheEnv returns the env vars for the breathe restart command.
func selectBreatheEnv(agent string, shellCfg *config.Config) []string {
	return buildBreatheEnv(agent, shellCfg)
}

// buildBreatheHandoff returns the composed handoff string and trigger for a breathe request.
// It evaluates breathe_context commands (falling back to diaryReadToday) then appends any
// route prompt/message. The trigger comes from the route request when present.
func buildBreatheHandoff(
	shellCfg *config.Config, req BreatheRequest, team string, routeReq *route.Request,
) (handoff, trigger string) {
	// Build base context from breathe_context commands or diary fallback.
	if cmds := shellCfg.BreatheContextCommands(); len(cmds) > 0 {
		ctx := evaluateBreatheContext(cmds, req.Agent, team)
		if ctx != "" {
			handoff = ctx
		} else {
			handoff = req.Handoff // all commands failed, use original
		}
	} else {
		handoff = diaryReadToday(req.Agent, req.Handoff) // backward compat
	}

	// Append route context if present.
	if routeReq == nil {
		return handoff, ""
	}
	if routeReq.RolePrompt != "" {
		handoff += "\n\n---\n\n## New Task Assignment\n\n" + routeReq.RolePrompt
	}
	if routeReq.Message != "" {
		handoff += "\n\n" + routeReq.Message
	}
	log.Printf("[breathe] routing %s to task %s (routed by %s)", req.Agent, routeReq.TaskUUID, routeReq.RoutedBy)
	return handoff, routeReq.Trigger
}

// handleBreathe restarts an agent's CC session with a handoff prompt.
// shellCfg is loaded once at daemon startup and passed in — never loaded per-request.
func handleBreathe(shellCfg *config.Config, req BreatheRequest) SendResponse {
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

	// 1. Consume route file EARLY to get routing context for handoff composition.
	routeReq, err := route.Consume(req.Agent)
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("consume routing file: %v", err)}
	}

	// 2. Resolve session names and CWD.
	plan, err := resolveBreatheSessions(req, team, shellCfg)
	if err != nil {
		return SendResponse{OK: false, Error: err.Error()}
	}

	// 3. Get model, CC version, and git branch.
	am := resolveAgentModel(team, req.Agent)
	gitBranch := gitutil.BranchName(plan.cwd)
	if gitBranch == "" {
		log.Printf("[breathe] %s: could not detect git branch for %s — leaving empty", req.Agent, plan.cwd)
	}

	// 4. Persist handoff to diary (write-side, stays).
	diaryAppendHandoff(req.Agent, req.Handoff)

	// 5. Build session context and compose with route if present.
	composedHandoff, trigger := buildBreatheHandoff(shellCfg, req, team, routeReq)

	// 6. Write synthetic JSONL session (BEFORE killing anything).
	projectDir, err := breathe.CCProjectDir(plan.cwd)
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("resolve CC project dir: %v", err)}
	}
	newSessionID, err := breathe.WriteSyntheticSession(projectDir, breathe.SessionConfig{
		CWD:       plan.cwd,
		CCVersion: am.ccVersion,
		GitBranch: gitBranch,
		Handoff:   composedHandoff,
	})
	if err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("write session: %v", err)}
	}

	// 7. Update status file with new session ID.
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

	// 8. Build restart command with env.
	ccCmd := buildCCRestartCmd(newSessionID, am.model, req.Agent, trigger)
	agentEnv := selectBreatheEnv(req.Agent, shellCfg)
	fullCmd := shellCfg.BuildEnvShellCommand(agentEnv, ccCmd)

	// 9. Kill old session, create new.
	log.Printf("[breathe] %s: restarting as %s in %s (model: %s)", req.Agent, plan.newSessionName, plan.cwd, am.model)
	if tmux.SessionExists(plan.oldSessionName) {
		if err := tmux.KillSession(plan.oldSessionName); err != nil {
			log.Printf("[breathe] %s: kill session warning (may already be dead): %v", req.Agent, err)
		}
	}

	if err := tmux.NewSession(plan.newSessionName, plan.windowName, plan.cwd, fullCmd); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("create session: %v", err)}
	}

	// Inject secrets into tmux session env for future commands.
	injectSecretsToSession(plan.newSessionName)

	log.Printf("[breathe] %s: fresh breath taken (session: %s)", req.Agent, plan.newSessionName)

	return SendResponse{OK: true}
}

// buildBreatheEnv returns the env var list for a breathe restart command.
// Mirrors buildManagerAgentEnv: agent identity, TASKRC, allowlisted .env secrets.
func buildBreatheEnv(agent string, cfg *config.Config) []string {
	vars := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agent),
	}
	if taskRC := cfg.TaskRC(); taskRC != "" {
		vars = append(vars, fmt.Sprintf("TASKRC=%s", taskRC))
	}
	// Inject allowlisted .env vars — tokens stay in daemon, not agent sessions.
	vars = append(vars, env.AllowedDotEnvParts()...)
	return vars
}

// diaryAppendHandoff persists the handoff to the agent's diary. It is a
// best-effort side effect — if the diary binary is not found or the append
// fails, a warning is logged and the caller continues unchanged.
func diaryAppendHandoff(agent, handoff string) {
	diaryPath, err := exec.LookPath("diary")
	if err != nil {
		log.Printf("[breathe] %s: diary binary not found — skipping diary persistence", agent)
		return
	}

	cmd := exec.Command(diaryPath, agent, "append")
	cmd.Stdin = bytes.NewBufferString(handoff)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[breathe] %s: diary append failed — %v: %s", agent, err, strings.TrimSpace(string(out)))
		return
	}
	log.Printf("[breathe] %s: diary handoff persisted", agent)
}

// diaryReadToday returns today's diary entry for the agent. If the diary
// binary is missing, the read fails, or the entry is empty (normal on the
// first breathe of the day), the original handoff is returned unchanged.
func diaryReadToday(agent, handoff string) string {
	diaryPath, err := exec.LookPath("diary")
	if err != nil {
		return handoff
	}

	cmd := exec.Command(diaryPath, agent, "read")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[breathe] %s: diary read failed — %v: %s", agent, err, strings.TrimSpace(string(out)))
		return handoff
	}
	if len(bytes.TrimSpace(out)) == 0 {
		log.Printf("[breathe] %s: diary entry empty today — using original handoff", agent)
		return handoff
	}

	return string(out)
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

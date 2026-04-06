package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/env"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/temenos"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// clearSettleDelay is the time to wait after sending /clear before sending
// the start trigger prompt. Allows CC's /clear to complete and the
// SessionStart hook to re-inject context before the trigger lands.
const clearSettleDelay = 500 * time.Millisecond

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
	rt := mcfg.RuntimeForAgent(ta.TeamName, ta.TeamPath, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: mcfg.Global.UserName(), Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	return fe.SendText(context.Background(), ta.AgentName, req.Message)
}

// handleTo delivers a message to an agent via its runtime adapter.
// Falls back to worker session delivery when the recipient is job_id:agent_name.
// Rejects bare hex UUIDs with a helpful error message.
// Human→worker messages are sent as bare text (no [agent from:] prefix).
func handleTo(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		// Try parseWorkerAddress for job_id:agent_name format
		if jobID, agentName, ok := parseWorkerAddress(req.To); ok {
			session, dispatched, err := dispatchToWorkerOrManager(
				mcfg, jobID, agentName, msgSvc, mcfg.Global.UserName(), req.Team, req.To, req.Message, nil)
			if err != nil {
				return err
			}
			if dispatched {
				logDispatch("human-to-worker", mcfg.Global.UserName(), req.To, session)
				return nil
			}
		}
		// Bare hex UUID — reject with helpful error
		if isBareWorkerHex(req.To) {
			return bareHexError(req.To)
		}
		return fmt.Errorf("unknown agent or worker %s", req.To)
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
	rt := mcfg.RuntimeForAgent(ta.TeamName, ta.TeamPath, req.To)
	persistMsg(msgSvc, message.CreateParams{
		Sender: "system", Recipient: req.To, Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	return deliverToAgent(registry, mcfg, frontends, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another.
// Falls back to worker session delivery when the recipient is job_id:agent_name.
// Rejects bare hex UUIDs with a helpful error message.
// The sender may also be a worker job_id:agent_name (e.g. from ttal alert in a worker session).

// isValidHexPrefix reports whether s (case-insensitive) is at least 8 lowercase hex characters.
func isValidHexPrefix(s string) bool {
	s = strings.ToLower(s)
	if len(s) < 8 {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// isBareWorkerHex reports whether s is a bare hex string (no colon) that would pass
// resolveWorker's format validation (8+ hex chars).
func isBareWorkerHex(s string) bool {
	if strings.Contains(s, ":") {
		return false
	}
	return isValidHexPrefix(s)
}

// bareHexError returns the "bare worker UUID not supported" error with a helpful example.
func bareHexError(got string) error {
	example := "abc12345:coder"
	if len(got) >= 8 {
		example = got[:8] + ":coder"
	}
	return fmt.Errorf("bare worker UUID not supported, use job_id:agent_name format (e.g. %s)", example)
}

// dispatchToWorkerOrManager attempts to dispatch a message to a worker session identified by
// jobID, falling back to the task owner's manager session window. Returns (session, true, nil) on
// worker dispatch, (session, true, nil) on manager fallback dispatch, ( "", false, nil) if the
// address does not match a worker format, or ("", false, error) on failure.
// The caller logs the dispatch. This reduces cyclomatic complexity in callers that need both paths.
func dispatchToWorkerOrManager(
	mcfg *config.DaemonConfig,
	jobID, agentName string,
	msgSvc *message.Service, sender, team, recipient string,
	msg string, rt *runtime.Runtime,
) (string, bool, error) {
	session, err := resolveWorker(jobID)
	if err == nil {
		return session, true, dispatchToWorkerImpl(msgSvc, session, agentName, message.CreateParams{
			Sender: sender, Recipient: "worker:" + recipient, Content: msg,
			Team: team, Channel: message.ChannelCLI, Runtime: rt,
		}, msg)
	}
	// Fall back to manager window — subagent results return to the task
	// owner's session after the worker session is gone. dispatchToWorkerImpl is
	// generic tmux send-keys delivery and works for both worker sessions
	// and manager windows.
	fallback, mgrErr := resolveManagerWindow(jobID, agentName, mcfg)
	if mgrErr != nil {
		return "", false, fmt.Errorf("unknown agent or worker %s: %w", recipient, err)
	}
	return fallback, true, dispatchToWorkerImpl(msgSvc, fallback, agentName, message.CreateParams{
		Sender: sender, Recipient: "worker:" + recipient, Content: msg,
		Team: team, Channel: message.ChannelCLI, Runtime: rt,
	}, msg)
}

// dispatchToWorkerImpl is the real dispatchToWorker implementation.
// Exposed as a var so it can be overridden in tests.
var dispatchToWorkerImpl = func(
	msgSvc *message.Service, session, windowName string, params message.CreateParams, text string,
) error {
	persistMsg(msgSvc, params)
	return deliverToWorker(session, windowName, text)
}

// logDispatch logs the appropriate dispatch message based on session type.
// kind is the base log prefix (e.g. "human-to-worker", "agent-to-worker").
// isWorker indicates whether the session is a worker session (w- prefix).
func logDispatch(kind, sender, to, session string) {
	if strings.HasPrefix(session, "w-") {
		log.Printf("[daemon] %s: %s → %s (%s)", kind, sender, to, session)
	} else {
		log.Printf("[daemon] %s: %s → %s:%s", kind, sender, to, session)
	}
}

//nolint:gocyclo // handleAgentToAgent is a message routing dispatcher with inherently many branches
func handleAgentToAgent(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	// Validate From.
	if isBareWorkerHex(req.From) {
		return bareHexError(req.From)
	}
	var fromTA *config.TeamAgent
	if jobID, _, ok := parseWorkerAddress(req.From); ok {
		if _, err := resolveWorker(jobID); err != nil {
			return fmt.Errorf("unknown agent or worker: %s", req.From)
		}
	} else if fromTA = resolveAgent(mcfg, req.Team, req.From); fromTA == nil {
		if _, err := resolveWorker(req.From); err != nil {
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
	senderTeamPath := ""
	if fromTA != nil {
		senderTeamPath = fromTA.TeamPath
	}
	if toTA == nil {
		// Try parseWorkerAddress for To: job_id:agent_name
		if jobID, agentName, ok := parseWorkerAddress(req.To); ok {
			rt := mcfg.RuntimeForAgent(senderTeam, senderTeamPath, req.From)
			session, dispatched, err := dispatchToWorkerOrManager(
				mcfg, jobID, agentName, msgSvc, req.From, senderTeam, req.To, msg, &rt)
			if err != nil {
				return err
			}
			if dispatched {
				// Determine which dispatch path was taken by checking if the session
				// matches the worker prefix (w-<hex8>). Manager sessions don't match.
				isWorker := strings.HasPrefix(session, "w-")
				if isWorker {
					log.Printf("[daemon] agent-to-worker: %s → %s (%s)", req.From, req.To, session)
				} else {
					log.Printf("[daemon] agent-to-manager-window: %s → %s:%s", req.From, req.To, session)
				}
				return nil
			}
		}
		// Bare hex UUID — reject with helpful error
		if isBareWorkerHex(req.To) {
			return bareHexError(req.To)
		}
		return fmt.Errorf("unknown agent or worker %s", req.To)
	}
	rt := mcfg.RuntimeForAgent(senderTeam, senderTeamPath, req.From)
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

// parseWorkerAddress returns the job ID and agent name from a worker address string.
// Returns true when s matches "<8+ hex chars>:<non-empty name>".
func parseWorkerAddress(s string) (jobID, agentName string, ok bool) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 || parts[1] == "" {
		return "", "", false
	}
	prefix := parts[0]
	if !isValidHexPrefix(prefix) {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// resolveWorkerImpl finds a tmux session for a worker identified by hex UUID prefix.
// Session names follow the format: w-{uuid[:8]}-{slug}.
// idPrefix must be at least 8 hex characters (case-insensitive).
func resolveWorkerImpl(idPrefix string) (string, error) {
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

// buildAgentRolesFn is the function used to discover agent roles from the team path.
// Package-level var for test injection.
var buildAgentRolesFn = buildAgentRoles

// exportTaskByHexIDFn is the function used to look up a task by hex UUID.
// Package-level var for test injection.
var exportTaskByHexIDFn = taskwarrior.ExportTaskByHexID

// windowExistsFn is the function used to check if a tmux window exists.
// Package-level var for test injection.
var windowExistsFn = tmux.WindowExists

// resolveWorker is the function used to find a worker tmux session by UUID prefix.
// Package-level var for test injection.
var resolveWorker = resolveWorkerImpl

// resolveManagerWindow is the function used to resolve the manager session window for a task.
// Package-level var for test injection.
var resolveManagerWindow = resolveManagerWindowImpl

// resolveManagerWindowImpl resolves the manager session window for a task's owner agent.
// It queries taskwarrior for the task by hex ID, finds the owner agent from task tags,
// resolves the manager session, and verifies the window exists.
// Returns (sessionName, nil) on success or ("", error) on failure.
func resolveManagerWindowImpl(jobID, windowName string, mcfg *config.DaemonConfig) (string, error) {
	team := mcfg.Global.TeamName()
	teamPath := mcfg.Global.TeamPath()
	task, err := exportTaskByHexIDFn(jobID, "")
	if err != nil {
		return "", fmt.Errorf("resolve manager window: task lookup: %w", err)
	}
	agentRoles := buildAgentRolesFn(teamPath)
	ownerAgent := findAgentTag(task.Tags, agentRoles)
	if ownerAgent == "" {
		return "", fmt.Errorf("resolve manager window: no owner agent tag on task %s", jobID)
	}
	session := config.AgentSessionName(team, ownerAgent)
	if !windowExistsFn(session, windowName) {
		return "", fmt.Errorf("resolve manager window: window %s not found in session %s", windowName, session)
	}
	return session, nil
}

// deliverToWorker sends a message to a worker's tmux session.
func deliverToWorker(session, windowName, text string) error {
	return tmux.SendKeys(session, windowName, text)
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
// mcpConfig, if non-empty, is appended via --mcp-config.
// Extracted for unit testing.
// Used by spawnCCSession (cold start) via --resume. For breathe restarts use buildCCFreshCmd.
func buildCCRestartCmd(sessionID, model, agent, trigger, mcpConfig string) string {
	cmd := fmt.Sprintf(
		"claude --resume %s --model %s --dangerously-skip-permissions --agent %s",
		sessionID, model, agent,
	)
	cmd = launchcmd.AppendMCPConfig(cmd, mcpConfig)
	if trigger != "" {
		escaped := strings.ReplaceAll(trigger, "'", "'\\''")
		cmd += fmt.Sprintf(" -- '%s'", escaped)
	}
	return cmd
}

// buildCCFreshCmd returns the claude command for a breathe restart without --resume.
// The CC SessionStart hook (ttal context) injects the session context at startup.
// trigger, if non-empty, is appended as a positional arg after --.
// mcpConfig, if non-empty, is appended via --mcp-config.
func buildCCFreshCmd(model, agent, trigger, mcpConfig string) string {
	cmd := fmt.Sprintf(
		"claude --model %s --dangerously-skip-permissions --agent %s",
		model, agent,
	)
	cmd = launchcmd.AppendMCPConfig(cmd, mcpConfig)
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

// handleBreathe sends a handoff to an agent's CC session via tmux.
// Context injection is handled by the CC SessionStart hook (ttal context) which
// evaluates breathe_context commands and consumes any pending route file.
// shellCfg is loaded once at daemon startup and passed in — never loaded per-request.
//
//nolint:gocyclo,lll
func handleBreathe(shellCfg *config.Config, req BreatheRequest, mcfg *config.DaemonConfig, registry *adapterRegistry) SendResponse {
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

	// Dispatch to codex handler if agent uses Codex runtime
	if mcfg != nil {
		if ta, ok := mcfg.FindAgentInTeam(team, req.Agent); ok {
			rt := mcfg.RuntimeForAgent(team, ta.TeamPath, req.Agent)
			if rt == runtime.Codex {
				return handleCodexBreathe(req, team, registry)
			}
		}
	}

	// 1. Resolve session names and CWD.
	plan, err := resolveBreatheSessions(req, team, shellCfg)
	if err != nil {
		return SendResponse{OK: false, Error: err.Error()}
	}

	// 2. Get model info.
	am := resolveAgentModel(team, req.Agent)

	// 3. Persist handoff to diary (write-side persistence).
	diaryAppendHandoff(req.Agent, req.Handoff)

	// 4. Update status file — clear session ID so the statusline hook populates the real ID.
	if err := status.WriteAgent(team, status.AgentStatus{
		Agent:               req.Agent,
		SessionID:           "", // cleared; CC SessionStart hook populates the real session ID
		ContextUsedPct:      0,
		ContextRemainingPct: 100,
		ModelID:             am.model,
		ModelName:           am.modelName,
		CCVersion:           am.ccVersion,
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		log.Printf("[breathe] warning: failed to write status for %s/%s: %v", team, req.Agent, err)
	}

	// 5. Breathe: prefer /clear on a live session (hook re-injects context without restart).
	// Fall back to kill+fresh-start when the session is dead.
	// Note: diaryAppendHandoff (step 3) runs unconditionally so the handoff is persisted
	// before both paths — /clear causes the source=clear hook to read the updated diary.
	sessionAlive := tmux.SessionExists(plan.oldSessionName)
	if sessionAlive {
		log.Printf("[breathe] %s: session alive — sending /clear (source=clear hook will re-inject context)", req.Agent)
		if err := tmux.SendKeys(plan.oldSessionName, plan.windowName, "/clear"); err != nil {
			log.Printf("[breathe] %s: /clear failed (%v), falling back to restart", req.Agent, err)
		} else {
			log.Printf("[breathe] %s: /clear sent, scheduling start trigger after %v", req.Agent, clearSettleDelay)
			go func() {
				time.Sleep(clearSettleDelay)
				if err := tmux.SendKeys(plan.oldSessionName, plan.windowName, "go"); err != nil {
					log.Printf("[breathe] %s: start trigger after /clear failed: %v", req.Agent, err)
				} else {
					log.Printf("[breathe] %s: start trigger sent", req.Agent)
				}
			}()
			// Return OK immediately: the start trigger is best-effort and sent async.
			// Callers must not assume the agent is ready to receive work at this point.
			return SendResponse{OK: true}
		}
	}

	// Session dead or /clear failed — full restart.
	// Reuse the shared manager MCP config file — token lifecycle is daemon-scoped, not per-breathe.
	mcpPath := temenos.ManagerMCPConfigPath()
	if mcpPath == "" {
		log.Printf("[breathe] %s: MCP config path unavailable — restarting without MCP access", req.Agent)
	}
	ccCmd := buildCCFreshCmd(am.model, req.Agent, "", mcpPath)
	agentEnv := buildBreatheEnv(req.Agent, shellCfg)
	fullCmd := shellCfg.BuildEnvShellCommand(agentEnv, ccCmd)

	log.Printf("[breathe] %s: restarting as %s in %s (model: %s)", req.Agent, plan.newSessionName, plan.cwd, am.model)
	if sessionAlive {
		if err := tmux.KillSession(plan.oldSessionName); err != nil {
			log.Printf("[breathe] %s: kill session warning (may already be dead): %v", req.Agent, err)
		}
	}

	if err := tmux.NewSession(plan.newSessionName, plan.windowName, plan.cwd, fullCmd); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("create session: %v", err)}
	}

	// Inject secrets into tmux session env for future commands.
	injectSecretsToSession(plan.newSessionName)

	log.Printf("[breathe] %s: fresh breath taken (restart, session: %s)", req.Agent, plan.newSessionName)

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

// handleCodexBreathe performs a breathe restart for a Codex agent.
// Creates a new thread (auto-injecting identity via developerInstructions) and sends
// the handoff as the first turn.
func handleCodexBreathe(req BreatheRequest, team string, registry *adapterRegistry) SendResponse {
	adapter, ok := registry.get(team, req.Agent)
	if !ok {
		return SendResponse{OK: false, Error: "codex adapter not found for " + req.Agent}
	}

	// Persist handoff to diary
	diaryAppendHandoff(req.Agent, req.Handoff)

	// Create a new thread — CreateSession auto-injects developerInstructions
	ctx := context.Background()
	if _, err := adapter.CreateSession(ctx); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("codex create session: %v", err)}
	}

	// Send handoff as first turn in the new thread
	if err := adapter.SendMessage(ctx, req.Handoff); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("codex send handoff: %v", err)}
	}

	log.Printf("[breathe] %s: codex breathe done (new thread, handoff sent)", req.Agent)
	return SendResponse{OK: true}
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

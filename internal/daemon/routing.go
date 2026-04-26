package daemon

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/tta-lab/ttal-cli/internal/addressee"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/launchcmd"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// All actors in ttal are agents — humans, AI managers, and workers alike.
// They differ along three orthogonal dimensions:
//
//   - identity: a unique alias (e.g. "neil", "yuki", "27ee75a8:coder")
//   - kind:     Human / Agent / Worker — what they are (AddresseeKind)
//   - channel:  Telegram/Matrix (Human), tmux send-keys (Agent, Worker)
//
// The sender never cares which kind the recipient is. `ttal send --to <alias>`
// works uniformly. The daemon picks the channel from addr.Kind:
//
//   Human  → frontend.SendText (Telegram chat_id / Matrix invite)
//   Agent  → tmux send-keys to the persistent agent session
//   Worker → tmux send-keys to the worker session, with manager-window fallback
//
// dispatchSend (agent-originated) and dispatchSystemSend (system-originated)
// both resolve via resolveAddressee and switch on Kind. handleTo (bare-shell
// path) follows the same pattern.

// resolveAddressee resolves a name to an addressee.
// Resolution order: HUMAN (humanfs) FIRST, then AGENT, then WORKER (jobid:agent).
func resolveAddressee(cfg *config.Config, name string) (*addressee.Addressee, error) {
	// Try human first (only if config is available)
	humansPath, humansErr := resolveHumansPathAndGet(name)

	// Return found human immediately
	if hfe, ok := humansErr.(*humanFoundError); ok {
		return &addressee.Addressee{Kind: addressee.KindHuman, Name: hfe.h.Alias, Human: hfe.h}, nil
	}

	// Try agent
	if ta, ok := cfg.FindAgent(name); ok {
		return &addressee.Addressee{Kind: addressee.KindAgent, Name: ta.AgentName, Agent: ta}, nil
	}

	// Try worker format
	if jobID, _, ok := parseWorkerAddress(name); ok {
		return &addressee.Addressee{Kind: addressee.KindWorker, Name: name, WorkerJobID: jobID}, nil
	}

	// Bare hex UUID
	if isBareWorkerHex(name) {
		return nil, bareHexError(name)
	}

	// Build known lists for error
	agentNames := make([]string, 0, len(cfg.Agents()))
	for _, a := range cfg.Agents() {
		agentNames = append(agentNames, a.AgentName)
	}

	humanAliases := resolveHumanAliases(humansPath)
	if humansErr != nil && humanAliases == nil {
		return nil, fmt.Errorf("cannot resolve addressee %q: humans.toml unreadable: %w", name, humansErr)
	}

	return nil, fmt.Errorf("unknown addressee: %s (known agents: %v; known humans: %v)",
		name, agentNames, humanAliases)
}

// resolveHumansPathAndGet tries to resolve a human by name and returns the path and any non-not-exist error.
func resolveHumansPathAndGet(name string) (string, error) {
	cfgPath, err := config.HumansPath()
	if err != nil || cfgPath == "" {
		return "", nil
	}
	h, err := humanfs.Get(cfgPath, name)
	if err == nil {
		return cfgPath, &humanFoundError{h: h}
	}
	if os.IsNotExist(err) {
		return cfgPath, nil
	}
	return cfgPath, err
}

// humanFoundError wraps a found human to distinguish it from not-found.
type humanFoundError struct {
	h *humanfs.Human
}

func (e *humanFoundError) Error() string { return "human found" }

// resolveHumanAliases lists human aliases, returning nil on not-exist.
func resolveHumanAliases(humansPath string) []string {
	if humansPath == "" {
		return nil
	}
	humansList, listErr := humanfs.List(humansPath)
	if listErr != nil && !os.IsNotExist(listErr) {
		return nil
	}
	var aliases []string
	for _, h := range humansList {
		aliases = append(aliases, h.Alias)
	}
	return aliases
}

// clearSettleDelay is the time to wait after sending /clear before sending
// the start trigger prompt. Allows CC's /clear to complete before the
// `Run ttal context` trigger lands. The agent runs ttal context themselves
// to get diary, agents, projects, pairing, role, and task.
const clearSettleDelay = 500 * time.Millisecond

var breatheStartTriggerFallback = launchcmd.ContextTrigger

// buildBreatheStartTrigger returns the wake-orientation trigger for the given agent.
// All wake paths emit the same trigger — ttal context is the single source of
// truth for wake orientation and renders diary, agents, projects, pairing, role,
// and task itself. Per-role skill inlining happens inside ttal pipeline prompt,
// which ttal context shells out to.
func buildBreatheStartTrigger(agentName string) string {
	if agentName == "" {
		log.Printf("[breathe] start trigger: empty agent name, using fallback")
	}
	return breatheStartTriggerFallback
}


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
	cfg *config.Config, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	// Remove the "human" literal case — humans are now first-class addressees.
	// dispatchSend handles all kinds via resolveAddressee + Kind switch.
	switch {
	case req.From == "system" && req.To != "":
		return dispatchSystemSend(cfg, registry, frontends, msgSvc, req)
	case req.From != "" && req.To != "":
		return dispatchSend(cfg, registry, frontends, msgSvc, req)
	case req.To != "":
		return handleTo(cfg, registry, frontends, msgSvc, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleTo delivers a message to an agent, worker, or human via resolveAddressee.
// Falls back to worker session delivery when the recipient is job_id:agent_name.
func handleTo(
	cfg *config.Config, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	addr, err := resolveAddressee(cfg, req.To)
	if err != nil {
		return err
	}

	switch addr.Kind {
	case addressee.KindHuman:
		if addr.Human == nil {
			return fmt.Errorf("addressee %s: KindHuman but Human field is nil", addr.Name)
		}
		return handleToHuman(frontends, msgSvc, addr.Human, req)
	case addressee.KindAgent:
		persistMsg(msgSvc, message.CreateParams{
			Sender: cfg.UserName, Recipient: addr.Name, Content: req.Message,
			Team: "default", Channel: message.ChannelCLI,
		})
		return deliverToAgent(registry, cfg, frontends, addr.Name, req.Message)
	case addressee.KindWorker:
		session, dispatched, err := dispatchToWorkerOrManager(
			cfg, addr.WorkerJobID, addr.Name, msgSvc, cfg.UserName, req.To, req.Message, nil)
		if err != nil {
			return err
		}
		if dispatched {
			logDispatch("human", cfg.UserName, req.To, session)
			return nil
		}
		return fmt.Errorf("worker %s not reachable", addr.Name)
	default:
		return fmt.Errorf("unknown addressee kind: %v", addr.Kind)
	}
}

// handleToHuman delivers a message to a human via the team's default frontend.
func handleToHuman(
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, human *humanfs.Human, req SendRequest,
) error {
	fe, ok := frontends["default"]
	if !ok {
		return fmt.Errorf("no frontend configured (human %s)", human.Alias)
	}
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: human.Alias, Content: req.Message,
		Team: "default", Channel: message.ChannelCLI,
	})
	fromAddr := &addressee.Addressee{Kind: addressee.KindAgent, Name: req.From}
	if req.From == "" || req.From == "system" {
		fromAddr = nil // frontend uses notification bot for non-agent senders
	}
	return fe.SendText(
		context.Background(),
		fromAddr,
		&addressee.Addressee{Kind: addressee.KindHuman, Name: human.Alias, Human: human},
		req.Message,
	)
}

// handleSystemToAgent delivers a system-originated message to an agent as bare text.
// No [agent from:] prefix is added — used for automated triggers like /breathe
// where CC must receive raw text to recognize it as a skill trigger.
// dispatchSystemSend delivers a system-originated message to its addressee.
// Bare text is preserved on the agent path (no [agent from:] prefix) — used
// for automated triggers like /breathe where CC must recognize raw text.
// No From validation — system sender is trusted via socket transport (daemon
// socket is 0o600 local-only). Future readers: don't add From checks here,
// that's intentional.
func dispatchSystemSend(
	cfg *config.Config, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	addr, err := resolveAddressee(cfg, req.To)
	if err != nil {
		return err
	}

	switch addr.Kind {
	case addressee.KindHuman:
		if addr.Human == nil {
			return fmt.Errorf("addressee %s: KindHuman but Human field is nil", addr.Name)
		}
		return handleToHuman(frontends, msgSvc, addr.Human, req)
	case addressee.KindAgent:
		rt := cfg.RuntimeForAgent(req.To)
		persistMsg(msgSvc, message.CreateParams{
			Sender: "system", Recipient: req.To, Content: req.Message,
			Team: defaultTeamName, Channel: message.ChannelCLI, Runtime: &rt,
		})
		return deliverToAgentFn(registry, cfg, frontends, addr.Name, req.Message)
	case addressee.KindWorker:
		jobID, agentName, _ := parseWorkerAddress(req.To)
		session, dispatched, err := dispatchToWorkerOrManager(
			cfg, jobID, agentName, msgSvc, "system", req.To, req.Message, nil)
		if err != nil {
			return err
		}
		if !dispatched {
			return fmt.Errorf("worker %s not reachable", req.To)
		}
		logDispatch("system", "system", req.To, session)
		return nil
	default:
		return fmt.Errorf("unknown addressee kind: %v", addr.Kind)
	}
}

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
	cfg *config.Config,
	jobID, agentName string,
	msgSvc *message.Service, sender, recipient string,
	msg string, rt *runtime.Runtime,
) (string, bool, error) {
	session, err := resolveWorker(jobID)
	if err == nil {
		return session, true, dispatchToWorkerImpl(msgSvc, session, agentName, message.CreateParams{
			Sender: sender, Recipient: "worker:" + recipient, Content: msg,
			Team: "default", Channel: message.ChannelCLI, Runtime: rt,
		}, msg)
	}
	// Fall back to manager window — subagent results return to the task
	// owner's session after the worker session is gone. dispatchToWorkerImpl is
	// generic tmux send-keys delivery and works for both worker sessions
	// and manager windows.
	fallback, mgrErr := resolveManagerWindow(jobID, agentName, cfg)
	if mgrErr != nil {
		return "", false, fmt.Errorf("unknown agent or worker %s: worker: %w; manager: %w", recipient, err, mgrErr)
	}
	return fallback, true, dispatchToWorkerImpl(msgSvc, fallback, agentName, message.CreateParams{
		Sender: sender, Recipient: "worker:" + recipient, Content: msg,
		Team: "default", Channel: message.ChannelCLI, Runtime: rt,
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

// logDispatch logs the dispatch message. source is the sender kind ("human" or "agent");
// the function infers the full label from the session type (worker vs manager window).
func logDispatch(source, sender, to, session string) {
	if strings.HasPrefix(session, "w-") {
		log.Printf("[daemon] %s-to-worker: %s → %s (%s)", source, sender, to, session)
	} else {
		log.Printf("[daemon] %s-to-manager-window: %s → %s:%s", source, sender, to, session)
	}
}

//nolint:gocyclo // dispatcher with inherently many branches
func dispatchSend(
	cfg *config.Config, registry *adapterRegistry,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service, req SendRequest,
) error {
	// Validate From — keep existing block verbatim.
	if isBareWorkerHex(req.From) {
		return bareHexError(req.From)
	}
	if jobID, _, ok := parseWorkerAddress(req.From); ok {
		if _, err := resolveWorker(jobID); err != nil {
			return fmt.Errorf("unknown agent or worker: %s", req.From)
		}
	} else if _, agentOK := cfg.FindAgent(req.From); !agentOK {
		if _, err := resolveWorker(req.From); err != nil {
			return fmt.Errorf("unknown agent or worker: %s", req.From)
		}
	}

	// Resolve To via the unified addressee resolver.
	addr, err := resolveAddressee(cfg, req.To)
	if err != nil {
		return err
	}

	msg := formatAgentMessage(req.From, req.Message)
	rt := cfg.RuntimeForAgent(req.From)

	switch addr.Kind {
	case addressee.KindHuman:
		if addr.Human == nil {
			return fmt.Errorf("addressee %s: KindHuman but Human field is nil", addr.Name)
		}
		return handleToHuman(frontends, msgSvc, addr.Human, SendRequest{
			From: req.From, To: req.To, Message: req.Message,
		})
	case addressee.KindAgent:
		persistMsg(msgSvc, message.CreateParams{
			Sender: req.From, Recipient: req.To, Content: req.Message,
			Team: defaultTeamName, Channel: message.ChannelCLI, Runtime: &rt,
		})
		log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
		return deliverToAgentFn(registry, cfg, frontends, addr.Name, msg)
	case addressee.KindWorker:
		jobID, agentName, _ := parseWorkerAddress(req.To)
		session, dispatched, err := dispatchToWorkerOrManager(
			cfg, jobID, agentName, msgSvc, req.From, req.To, msg, &rt)
		if err != nil {
			return err
		}
		if !dispatched {
			return fmt.Errorf("worker %s not reachable", req.To)
		}
		logDispatch("agent", req.From, req.To, session)
		return nil
	default:
		return fmt.Errorf("unknown addressee kind: %v", addr.Kind)
	}
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

// exportTaskByHexIDFn is the function used to look up a task by hex UUID.
// Package-level var for test injection.
var exportTaskByHexIDFn = taskwarrior.ExportTaskByHexID

// windowExistsFn is the function used to check if a tmux window exists.
// Package-level var for test injection.
var windowExistsFn = tmux.WindowExists

// tmuxSendKeysFn sends keys to a tmux session window.
// Package-level var for test injection.
var tmuxSendKeysFn = tmux.SendKeys

// tmuxSessionExistsFn checks if a tmux session exists.
// Package-level var for test injection.
var tmuxSessionExistsFn = tmux.SessionExists

// resolveWorker is the function used to find a worker tmux session by UUID prefix.
// Package-level var for test injection.
var resolveWorker = resolveWorkerImpl

// resolveManagerWindow is the function used to resolve the manager session window for a task.
// Package-level var for test injection.
var resolveManagerWindow = resolveManagerWindowImpl

// deliverToAgentFn delivers a message to an agent session.
// Package-level var for test injection.
var deliverToAgentFn = deliverToAgent

// pipelineLoadFn is the function used to load pipeline config.
// Package-level var for test injection.
var pipelineLoadFn = pipeline.Load

// resolveManagerWindowImpl resolves the manager session window for a task's owner agent.
// Reads task.Owner directly (write-once, set at first manager-stage routing).
// Returns (sessionName, nil) on success or ("", error) on failure.
// Returns an error if the task is at a worker stage — callers should route to the worker session instead.
func resolveManagerWindowImpl(jobID, windowName string, cfg *config.Config) (string, error) {
	task, err := exportTaskByHexIDFn(jobID, "")
	if err != nil {
		return "", fmt.Errorf("resolve manager window: task lookup: %w", err)
	}
	if task.Owner == "" {
		return "", fmt.Errorf("resolve manager window: no owner on task %s", jobID)
	}

	// Load pipeline config to determine current stage.
	pipeCfg, err := pipelineLoadFn(config.DefaultConfigDir())
	if err != nil {
		return "", fmt.Errorf("resolve manager window: load pipeline config: %w", err)
	}
	_, p, err := pipeCfg.MatchPipeline(task.Tags)
	if err != nil {
		return "", fmt.Errorf("resolve manager window: match pipeline: %w", err)
	}
	if p == nil {
		return "", fmt.Errorf("resolve manager window: no pipeline matches task tags %v", task.Tags)
	}
	_, stage, err := p.CurrentStage(task.Tags)
	if err != nil {
		return "", fmt.Errorf("resolve manager window: current stage: %w", err)
	}
	if stage != nil && stage.IsWorker() {
		return "", fmt.Errorf("resolve manager window: task at worker stage, no manager window for %s", jobID)
	}

	session := config.AgentSessionName(task.Owner)
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
func resolveAgentModel(agent string) breatheAgentModel {
	s, err := status.ReadAgent("default", agent)
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

// diaryAppendHandoff persists the handoff to the agent's diary.
// It prefers the live pane CWD; if the session is dead or pane CWD is unavailable,
// it falls back to the registered agent workspace path from config.
// Returns sessionAlive so the caller can skip KillSession on a dead session.
func resolveBrCWD(sessionName, windowName, agent string, cfg *config.Config) (string, bool, error) {
	var cwd string
	sessionAlive := tmuxSessionExistsFn(sessionName)
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
	req BreatheRequest, shellCfg *config.Config,
) (breatheSessionPlan, error) {
	persistName := config.AgentSessionName(req.Agent)
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
// On restart the agent's spawn trigger says `Run ttal context for your briefing`,
// and the agent invokes ttal context themselves to render diary + role + task.
// shellCfg is loaded once at daemon startup and passed in — never loaded per-request.
//
//nolint:gocyclo,lll
func handleBreathe(shellCfg *config.Config, req BreatheRequest, cfg *config.Config, registry *adapterRegistry) SendResponse {
	if req.Agent == "" {
		return SendResponse{OK: false, Error: "missing agent name"}
	}
	if req.Handoff == "" {
		return SendResponse{OK: false, Error: "empty handoff prompt"}
	}

	// Dispatch to codex handler if agent uses Codex runtime
	if cfg != nil {
		if _, ok := cfg.FindAgent(req.Agent); ok {
			rt := cfg.RuntimeForAgent(req.Agent)
			if rt == runtime.Codex {
				return handleCodexBreathe(req, registry)
			}
		}
	}

	// 1. Resolve session names and CWD.
	plan, err := resolveBreatheSessions(req, shellCfg)
	if err != nil {
		return SendResponse{OK: false, Error: err.Error()}
	}

	// 2. Get model info.
	am := resolveAgentModel(req.Agent)

	// 3. Persist handoff to diary (write-side persistence).
	diaryAppendHandoff(req.Agent, req.Handoff)

	// 4. Update status file — clear session ID so the statusline hook populates the real ID.
	if err := status.WriteAgent("default", status.AgentStatus{
		Agent:               req.Agent,
		SessionID:           "", // cleared; CC SessionStart hook populates the real session ID
		ContextUsedPct:      0,
		ContextRemainingPct: 100,
		ModelID:             am.model,
		ModelName:           am.modelName,
		CCVersion:           am.ccVersion,
		UpdatedAt:           time.Now().UTC(),
	}); err != nil {
		log.Printf("[breathe] warning: failed to write status for default/%s: %v", req.Agent, err)
	}

	// 5. Breathe: prefer /clear on a live session (hook re-injects context without restart).
	// Fall back to kill+fresh-start when the session is dead.
	// Note: diaryAppendHandoff (step 3) runs unconditionally so the handoff is persisted
	// before both paths — /clear causes the source=clear hook to read the updated diary.
	sessionAlive := tmuxSessionExistsFn(plan.oldSessionName)
	if sessionAlive {
		log.Printf("[breathe] %s: session alive — sending /clear (source=clear hook will re-inject context)", req.Agent)
		if err := tmuxSendKeysFn(plan.oldSessionName, plan.windowName, "/clear"); err != nil {
			log.Printf("[breathe] %s: /clear failed (%v), falling back to restart", req.Agent, err)
		} else {
			log.Printf("[breathe] %s: /clear sent, scheduling start trigger after %v", req.Agent, clearSettleDelay)
			go func() {
				time.Sleep(clearSettleDelay)
				trigger := buildBreatheStartTrigger(req.Agent)
				if err := tmuxSendKeysFn(plan.oldSessionName, plan.windowName, trigger); err != nil {
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

	// Session dead or /clear failed — full restart via spawnCCSession.
	log.Printf("[breathe] %s: restarting as %s in %s (model: %s)", req.Agent, plan.newSessionName, plan.cwd, am.model)
	if sessionAlive {
		if err := tmux.KillSession(plan.oldSessionName); err != nil {
			log.Printf("[breathe] %s: kill session warning (may already be dead): %v", req.Agent, err)
		}
	}
	agentEnv := buildManagerAgentEnv(req.Agent, cfg)
	if err := spawnCCSession(plan.newSessionName, req.Agent, plan.cwd, agentEnv, shellCfg.GetShell(), ""); err != nil {
		return SendResponse{OK: false, Error: fmt.Sprintf("create session: %v", err)}
	}
	log.Printf("[breathe] %s: fresh breath taken (restart, session: %s)", req.Agent, plan.newSessionName)

	return SendResponse{OK: true}
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
func handleCodexBreathe(req BreatheRequest, registry *adapterRegistry) SendResponse {
	adapter, ok := registry.get("default", req.Agent)
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
	s := status.AgentStatus{
		Agent:               req.Agent,
		ContextUsedPct:      req.ContextUsedPct,
		ContextRemainingPct: req.ContextRemainingPct,
		ModelID:             req.ModelID,
		SessionID:           req.SessionID,
		UpdatedAt:           time.Now(),
	}
	if err := status.WriteAgent("default", s); err != nil {
		log.Printf("[daemon] failed to write status for default/%s: %v", req.Agent, err)
	}
}

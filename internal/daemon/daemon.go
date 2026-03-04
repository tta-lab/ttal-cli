package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/runtime/codex"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

const pidFileName = "daemon.pid"

// restartCh is signaled when a /restart command is received.
var restartCh = make(chan struct{})

// pollerTarget groups agent info for Telegram poller dispatch by chat ID.
type pollerTarget struct {
	teamName  string
	agentName string
	chatID    string
	agentCfg  config.AgentConfig
}

// Run starts the daemon in the foreground. This is what launchd calls.
// Config-driven: loads all teams from config.toml, no database required.
func Run() error {
	mcfg, err := config.LoadAll()
	if err != nil {
		return err
	}

	if running, pid, _ := IsRunning(); running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}

	pidPath, err := setupDataDir()
	if err != nil {
		return err
	}
	defer os.Remove(pidPath)

	sockPath, err := SocketPath()
	if err != nil {
		return err
	}

	allAgents := mcfg.AllAgents()
	log.Printf("[daemon] starting — socket=%s teams=%d agents=%d",
		sockPath, len(mcfg.Teams), len(allAgents))

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := newAdapterRegistry()
	qs := newQuestionStore()
	cas := newCustomAnswerStore()
	mt := newMessageTracker()

	// Run adapter init and bot command registration concurrently — they're independent.
	var startupWg sync.WaitGroup
	var allCommands []BotCommand

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		initAdapters(ctx, mcfg, registry, qs, mt)
	}()

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		allCommands = discoverAndRegisterCommands(mcfg, allAgents)
	}()

	startupWg.Wait()
	startTelegramPollers(mcfg, allAgents, registry, done, qs, cas, allCommands, mt)
	startCleanupWatcher(done)
	startPRWatcher(mcfg, registry, done)
	startWatcherIfNeeded(mcfg, allAgents, qs, mt, done)

	cleanup, err := listenSocket(sockPath, socketHandlers{
		send: func(req SendRequest) error {
			return handleSend(mcfg, registry, req)
		},
		statusUpdate: handleStatusUpdate,
	})
	if err != nil {
		close(done)
		return err
	}

	go runQuestionCleanup(qs, done)

	log.Printf("[daemon] ready")
	notifyDaemonReady(mcfg)
	awaitShutdown(done, cancel, mcfg, registry, cleanup)
	return nil
}

// setupDataDir creates ~/.ttal/ and writes the PID file. Returns the PID file path.
func setupDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dataDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", err
	}
	pidPath := filepath.Join(dataDir, pidFileName)
	if err := writePID(pidPath); err != nil {
		return "", fmt.Errorf("failed to write pid file: %w", err)
	}
	return pidPath, nil
}

// discoverAndRegisterCommands discovers dynamic commands and registers them with Telegram bots.
func discoverAndRegisterCommands(mcfg *config.DaemonConfig, allAgents []config.TeamAgent) []BotCommand {
	discovered := DiscoverCommands(mcfg.Global.Sync.CommandsPaths)
	allCommands := AllCommands(discovered)
	log.Printf("[daemon] discovered %d dynamic commands", len(discovered))

	// Deduplicate tokens first
	tokenAgent := make(map[string]string) // token -> first agent name (for logging)
	for _, ta := range allAgents {
		if ta.Config.BotToken == "" {
			continue
		}
		if _, ok := tokenAgent[ta.Config.BotToken]; !ok {
			tokenAgent[ta.Config.BotToken] = ta.AgentName
		}
	}

	var wg sync.WaitGroup
	for token, agentName := range tokenAgent {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := RegisterBotCommands(token, allCommands); err != nil {
				log.Printf("[daemon] warning: failed to register bot commands for %s: %v",
					agentName, err)
			} else {
				log.Printf("[daemon] registered bot commands for %s", agentName)
			}
		}()
	}
	wg.Wait()
	return allCommands
}

// startTelegramPollers deduplicates agents by bot token and starts one poller per token.
func startTelegramPollers(
	mcfg *config.DaemonConfig, allAgents []config.TeamAgent,
	registry *adapterRegistry, done chan struct{},
	qs *questionStore, cas *customAnswerStore, allCommands []BotCommand,
	mt *messageTracker,
) {
	tokenTargets := buildTokenTargets(mcfg, allAgents)

	for botToken, targets := range tokenTargets {
		dispatchMap := buildDispatchMap(targets)
		log.Printf("[daemon] starting multi-agent poller for %d agents on token ...%s",
			len(targets), botToken[len(botToken)-min(4, len(botToken)):])
		startMultiAgentPoller(botToken, dispatchMap, func(teamName, agentName, text string) {
			if err := deliverToAgent(registry, mcfg, teamName, agentName, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", agentName, err)
			}
		}, done, qs, cas, registry, allCommands, mt)
	}
}

// buildTokenTargets groups agents by bot token, skipping those without tokens or using OpenClaw.
func buildTokenTargets(mcfg *config.DaemonConfig, allAgents []config.TeamAgent) map[string][]pollerTarget {
	tokenTargets := make(map[string][]pollerTarget)
	for _, ta := range allAgents {
		if ta.Config.BotToken == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", ta.AgentName)
			continue
		}
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
		if rt == runtime.OpenClaw {
			log.Printf("[daemon] agent %s uses OpenClaw — skipping Telegram poller", ta.AgentName)
			continue
		}
		tokenTargets[ta.Config.BotToken] = append(tokenTargets[ta.Config.BotToken], pollerTarget{
			teamName:  ta.TeamName,
			agentName: ta.AgentName,
			chatID:    ta.ChatID,
			agentCfg:  ta.Config,
		})
	}
	return tokenTargets
}

// buildDispatchMap converts poller targets into a chat ID → target map.
func buildDispatchMap(targets []pollerTarget) map[int64]pollerTarget {
	dispatchMap := make(map[int64]pollerTarget)
	for _, t := range targets {
		chatID, err := telegram.ParseChatID(t.chatID)
		if err != nil {
			log.Printf("[daemon] invalid chat_id for %s: %v", t.agentName, err)
			continue
		}
		if existing, ok := dispatchMap[chatID]; ok {
			log.Printf("[daemon] WARNING: chat ID %d collision — "+
				"agent %s/%s overwrites %s/%s (same bot token, same chat)",
				chatID, t.teamName, t.agentName, existing.teamName, existing.agentName)
		}
		dispatchMap[chatID] = t
	}
	return dispatchMap
}

// startWatcherIfNeeded starts the JSONL watcher unless all agents use OpenClaw.
func startWatcherIfNeeded(
	mcfg *config.DaemonConfig, allAgents []config.TeamAgent,
	qs *questionStore, mt *messageTracker, done <-chan struct{},
) {
	for _, ta := range allAgents {
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
		if rt != runtime.OpenClaw {
			startWatcher(mcfg, qs, mt, done)
			return
		}
	}
	log.Printf("[daemon] all agents use OpenClaw — skipping JSONL watcher")
}

// runQuestionCleanup periodically cleans up stale question batches.
func runQuestionCleanup(qs *questionStore, done <-chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			qs.cleanup(30 * time.Minute)
		}
	}
}

// notifyDaemonReady sends a startup notification to the default team via its notification bot token.
func notifyDaemonReady(mcfg *config.DaemonConfig) {
	defaultTeam := mcfg.DefaultTeamName()
	team, ok := mcfg.Teams[defaultTeam]
	if !ok {
		log.Printf("[daemon] warning: default team %q not found in config", defaultTeam)
		return
	}
	if err := notify.SendWithConfig(team.NotificationToken, team.ChatID, "✅ Daemon ready"); err != nil {
		log.Printf("[daemon] warning: failed to send ready notification: %v", err)
	}
}

// awaitShutdown waits for SIGINT/SIGTERM and performs graceful shutdown.
func awaitShutdown(
	done chan struct{}, cancel context.CancelFunc,
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	cleanup func(),
) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	select {
	case s := <-sig:
		log.Printf("[daemon] received signal %v — shutting down", s)
	case <-restartCh:
		log.Printf("[daemon] restart requested via Telegram — shutting down")
	}
	close(done)
	cancel()
	shutdownAgents(mcfg, registry)
	cleanup()
}

// initAdapters starts all agent sessions in parallel: tmux for CC, HTTP adapters for OC/Codex/OpenClaw.
// Config-driven: iterates all teams, no DB required.
func initAdapters(
	ctx context.Context, mcfg *config.DaemonConfig,
	registry *adapterRegistry, qs *questionStore, mt *messageTracker,
) {
	var wg sync.WaitGroup
	for _, ta := range mcfg.AllAgents() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initSingleAdapter(ctx, ta, mcfg, registry, qs, mt)
		}()
	}
	wg.Wait()
}

// initSingleAdapter initializes a single agent: tmux session for CC, HTTP adapter for others.
func initSingleAdapter(
	ctx context.Context, ta config.TeamAgent, mcfg *config.DaemonConfig,
	registry *adapterRegistry, qs *questionStore, mt *messageTracker,
) {
	agentPath := filepath.Join(ta.TeamPath, ta.AgentName)
	rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)

	// CC agents use tmux — spawn session but don't register adapter
	// (deliverToAgent falls back to tmux send-keys for unregistered agents).
	if rt == runtime.ClaudeCode {
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if tmux.SessionExists(sessionName) {
			log.Printf("[daemon] CC agent %s already running (session: %s)", ta.AgentName, sessionName)
			return
		}
		model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
		env := buildAgentEnv(ta.AgentName, ta.TeamName, mcfg)
		if err := spawnCCSession(sessionName, ta.AgentName, agentPath, model, env, mcfg.Global.GetShell()); err != nil {
			log.Printf("[daemon] failed to start CC session for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] CC agent %s running (session: %s)", ta.AgentName, sessionName)
		}
		return
	}

	model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
	port := ta.Config.Port
	if rt.NeedsPort() && port == 0 {
		log.Printf("[daemon] skipping %s adapter for %s/%s: "+
			"port not configured (set [teams.%s.agents.%s] port = N)",
			rt, ta.TeamName, ta.AgentName,
			ta.TeamName, ta.AgentName)
		return
	}
	env := buildAgentEnv(ta.AgentName, ta.TeamName, mcfg)

	team := mcfg.Teams[ta.TeamName]
	adapter := createAdapterFromTeam(ta.AgentName, rt, agentPath, port, model, true, env, team)
	if err := adapter.Start(ctx); err != nil {
		log.Printf("[daemon] failed to start %s adapter for %s: %v", rt, ta.AgentName, err)
		return
	}
	registry.set(ta.TeamName, ta.AgentName, adapter)
	log.Printf("[daemon] started %s adapter for %s on port %d", rt, ta.AgentName, port)
	// Create or resume session for adapters that need one.
	if rt == runtime.OpenCode || rt == runtime.Codex {
		initSession(ctx, rt, ta.AgentName, adapter)
	}
	// OpenClaw owns messaging — skip Telegram event bridging
	if rt != runtime.OpenClaw {
		bridgeEvents(ta.AgentName, ta.TeamName, adapter, mcfg, qs, mt)
	}
}

// initSession creates or resumes a session for the adapter.
// For Codex, it tries to resume the most recent thread (like CC's --continue)
// before falling back to creating a new one.
func initSession(ctx context.Context, rt runtime.Runtime, agentName string, adapter runtime.Adapter) {
	var sid string
	if rt == runtime.Codex {
		if ca, ok := adapter.(*codex.Adapter); ok {
			sid = tryResumeCodexThread(ctx, ca, agentName)
		}
	}
	if sid != "" {
		return
	}
	sid, err := adapter.CreateSession(ctx)
	if err != nil {
		log.Printf("[daemon] failed to create session for %s: %v", agentName, err)
	} else {
		log.Printf("[daemon] created session %s for %s", sid, agentName)
	}
}

// tryResumeCodexThread finds and resumes the most recent Codex thread.
// Returns the thread ID on success, empty string otherwise.
// If the resumed session has a stale approval policy (!= "never"), it falls back
// to creating a new session so the agent doesn't hang on approval requests.
func tryResumeCodexThread(ctx context.Context, ca *codex.Adapter, agentName string) string {
	lastID, err := ca.ListThreads(ctx)
	if err != nil {
		log.Printf("[daemon] failed to list threads for %s: %v", agentName, err)
		return ""
	}
	if lastID == "" {
		return ""
	}
	policy, err := ca.ResumeSession(ctx, lastID)
	if err != nil {
		log.Printf("[daemon] failed to resume %s for %s: %v — creating new", lastID, agentName, err)
		return ""
	}
	if policy != "" && policy != "never" {
		log.Printf("[daemon] resumed thread %s for %s has approvalPolicy=%q (want never), creating fresh session",
			lastID, agentName, policy)
		return "" // caller will create new session with approvalPolicy: "never"
	}
	log.Printf("[daemon] resumed session %s for %s", lastID, agentName)
	return lastID
}

// buildAgentEnv returns env vars for an agent adapter.
func buildAgentEnv(agentName, teamName string, mcfg *config.DaemonConfig) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
	}
	env = append(env, fmt.Sprintf("TTAL_TEAM=%s", teamName))
	if team, ok := mcfg.Teams[teamName]; ok && team.TaskRC != "" {
		env = append(env, fmt.Sprintf("TASKRC=%s", team.TaskRC))
	}

	// Inject all secrets from .env
	env = append(env, config.DotEnvParts()...)

	return env
}

// bridgeEvents reads events from an adapter and routes them to Telegram.
func bridgeEvents(
	agentName, teamName string, adapter runtime.Adapter,
	mcfg *config.DaemonConfig, qs *questionStore, mt *messageTracker,
) {
	ta, ok := mcfg.FindAgentInTeam(teamName, agentName)
	if !ok || ta.Config.BotToken == "" {
		return
	}
	chatID := ta.ChatID

	go func() {
		for event := range adapter.Events() {
			switch event.Type {
			case runtime.EventText:
				if err := telegram.SendMessage(ta.Config.BotToken, chatID, event.Text); err != nil {
					log.Printf("[daemon] telegram send error for %s: %v", agentName, err)
				}
			case runtime.EventError:
				log.Printf("[daemon] runtime error for %s: %s", agentName, event.Text)
			case runtime.EventQuestion:
				handleIncomingQuestion(qs, teamName, agentName, adapter.Runtime(), event.CorrelationID, event.Questions, mcfg)
			case runtime.EventTool:
				emoji := telegram.ToolEmoji(event.ToolName)
				if emoji == "" || mt == nil {
					break
				}
				tracked, ok := mt.get(teamName, agentName)
				if !ok {
					break
				}
				if err := telegram.SetReaction(tracked.BotToken, tracked.ChatID, tracked.MessageID, emoji); err != nil {
					log.Printf("[reactions] tool reaction error for %s (%s): %v", agentName, event.ToolName, err)
				}
			}
		}
	}()
}

// handleSend routes an incoming SendRequest based on From/To fields.
// Resolves team from agent name or the Team field in the request.
func handleSend(mcfg *config.DaemonConfig, registry *adapterRegistry, req SendRequest) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(mcfg, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgent(mcfg, registry, req)
	case req.From != "":
		return handleFrom(mcfg, req)
	case req.To != "":
		return handleTo(mcfg, registry, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleFrom sends a message from an agent to the human via Telegram.
func handleFrom(mcfg *config.DaemonConfig, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.From)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if ta.Config.BotToken == "" {
		return fmt.Errorf("agent %s has no telegram configured", req.From)
	}
	return telegram.SendMessage(ta.Config.BotToken, ta.ChatID, req.Message)
}

// handleTo delivers a message to an agent via its runtime adapter.
func handleTo(mcfg *config.DaemonConfig, registry *adapterRegistry, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	return deliverToAgent(registry, mcfg, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another.
func handleAgentToAgent(mcfg *config.DaemonConfig, registry *adapterRegistry, req SendRequest) error {
	fromTA := resolveAgent(mcfg, req.Team, req.From)
	if fromTA == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	toTA := resolveAgent(mcfg, req.Team, req.To)
	if toTA == nil {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	msg := formatAgentMessage(req.From, req.Message)
	log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
	return deliverToAgent(registry, mcfg, toTA.TeamName, req.To, msg)
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

// startWatcher initializes the JSONL watcher from config (all teams).
func startWatcher(mcfg *config.DaemonConfig, qs *questionStore, mt *messageTracker, done <-chan struct{}) {
	agentMap := make(map[string]watcher.AgentInfo)
	for _, ta := range mcfg.AllAgents() {
		agentPath := filepath.Join(ta.TeamPath, ta.AgentName)
		encoded := watcher.EncodePath(agentPath)
		agentMap[encoded] = watcher.AgentInfo{
			TeamName:  ta.TeamName,
			AgentName: ta.AgentName,
		}
	}

	w, err := watcher.New(agentMap,
		func(teamName, agentName, text string) {
			ta, ok := mcfg.FindAgentInTeam(teamName, agentName)
			if !ok || ta.Config.BotToken == "" {
				return
			}
			// Clear tracking — response text arriving is the done signal
			mt.delete(teamName, agentName)
			if err := telegram.SendMessage(ta.Config.BotToken, ta.ChatID, text); err != nil {
				log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
			}
		},
		func(teamName, agentName, correlationID string, questions []runtime.Question) {
			handleIncomingQuestion(qs, teamName, agentName, runtime.ClaudeCode, correlationID, questions, mcfg)
		},
		func(teamName, agentName, toolName string) {
			emoji := telegram.ToolEmoji(toolName)
			if emoji == "" {
				return
			}
			tracked, ok := mt.get(teamName, agentName)
			if !ok {
				return
			}
			if err := telegram.SetReaction(tracked.BotToken, tracked.ChatID, tracked.MessageID, emoji); err != nil {
				log.Printf("[reactions] tool reaction error for %s (%s): %v", agentName, toolName, err)
			}
		},
	)
	if err != nil {
		log.Printf("[daemon] watcher disabled: %v — CC→Telegram bridging will not work", err)
		return
	}
	go func() {
		if err := w.Run(done); err != nil {
			log.Printf("[daemon] watcher error: %v", err)
		}
	}()
}

// handleIncomingQuestion handles questions with team context.
func handleIncomingQuestion(
	store *questionStore,
	teamName, agentName string,
	rt runtime.Runtime,
	correlationID string,
	questions []runtime.Question,
	mcfg *config.DaemonConfig,
) {
	if len(questions) == 0 {
		return
	}

	for _, q := range questions {
		if q.MultiSelect {
			log.Printf("[questions] warning: multi-select not supported in Telegram UI"+
				" for %s question %q — treating as single-select", agentName, q.Header)
		}
	}

	ta, ok := mcfg.FindAgentInTeam(teamName, agentName)
	if !ok || ta.Config.BotToken == "" {
		log.Printf("[questions] no bot config for agent %s, dropping question", agentName)
		return
	}
	chatID, err := telegram.ParseChatID(ta.ChatID)
	if err != nil {
		log.Printf("[questions] invalid chat ID for %s: %v", agentName, err)
		return
	}

	batch := &QuestionBatch{
		ShortID:       store.nextShortID(),
		CorrelationID: correlationID,
		TeamName:      teamName,
		AgentName:     agentName,
		Runtime:       rt,
		Questions:     questions,
		Answers:       make(map[int]string),
		CurrentPage:   0,
		ChatID:        chatID,
		BotToken:      ta.Config.BotToken,
		CreatedAt:     time.Now(),
	}

	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)

	msgID, err := telegram.SendQuestionMessage(ta.Config.BotToken, chatID, text, markup)
	if err != nil {
		log.Printf("[questions] failed to send question to Telegram for %s: %v", agentName, err)
		return
	}
	batch.TelegramMsgID = msgID

	store.store(batch)
	log.Printf("[questions] sent question %s for %s (batch %s)", correlationID, agentName, batch.ShortID)
}

// IsRunning checks whether the daemon is running by inspecting the pid file.
// Uses fixed path at ~/.ttal/daemon.pid.
func IsRunning() (bool, int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, 0, fmt.Errorf("user home dir: %w", err)
	}
	pidPath := filepath.Join(home, ".ttal", pidFileName)
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return false, 0, nil
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return false, 0, nil
	}

	// Check if process is alive
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false, 0, nil
	}

	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return false, pid, nil
	}

	return true, pid, nil
}

// shutdownAgents kills all agent sessions on daemon exit.
func shutdownAgents(mcfg *config.DaemonConfig, registry *adapterRegistry) {
	// Stop all adapter-managed agents (OC/Codex/OpenClaw)
	registry.stopAll(context.Background())

	// Kill CC tmux sessions
	for _, ta := range mcfg.AllAgents() {
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
		if rt != runtime.ClaudeCode {
			continue
		}
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if !tmux.SessionExists(sessionName) {
			continue
		}
		if err := tmux.KillSession(sessionName); err != nil {
			log.Printf("[daemon] failed to kill session %s: %v", sessionName, err)
		} else {
			log.Printf("[daemon] killed CC session %s", sessionName)
		}
	}
}

// spawnCCSession creates a tmux session for a Claude Code agent.
func spawnCCSession(sessionName, agentName, agentPath, model string, env []string, shell string) error {
	cmd := "claude --dangerously-skip-permissions"
	if model != "" {
		cmd += " --model " + model
	}
	if hasCCConversation(agentPath) {
		cmd += " --continue"
	}

	envStr := ""
	if len(env) > 0 {
		envStr = fmt.Sprintf("env %s ", strings.Join(env, " "))
	}
	var shellCmd string
	switch shell {
	case "fish":
		shellCmd = fmt.Sprintf("%sfish -C '%s'", envStr, cmd)
	default:
		shellCmd = fmt.Sprintf("%szsh -c '%s'", envStr, cmd)
	}

	if err := tmux.NewSession(sessionName, agentName, agentPath, shellCmd); err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			_ = tmux.SetEnv(sessionName, parts[0], parts[1])
		}
	}
	return nil
}

// hasCCConversation checks if Claude Code has a previous conversation for the given path.
// Claude sanitizes paths: / and . are replaced with - to form the project directory name.
func hasCCConversation(workDir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	sanitized := strings.ReplaceAll(workDir, string(filepath.Separator), "-")
	sanitized = strings.ReplaceAll(sanitized, ".", "-")
	projectDir := filepath.Join(home, ".claude", "projects", sanitized)
	matches, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	return len(matches) > 0
}

func writePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

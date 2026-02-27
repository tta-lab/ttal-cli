package daemon

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"codeberg.org/clawteam/ttal-cli/internal/watcher"
)

const pidFileName = "daemon.pid"

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

	// Refuse to start if already running
	if running, pid, _ := IsRunning(); running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}

	// Fixed data dir at ~/.ttal/
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dataDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}

	// Write pid file
	pidPath := filepath.Join(dataDir, pidFileName)
	if err := writePID(pidPath); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}
	defer os.Remove(pidPath)

	sockPath, err := SocketPath()
	if err != nil {
		return err
	}

	allAgents := mcfg.AllAgents()
	log.Printf("[daemon] starting — socket=%s teams=%d agents=%d", sockPath, len(mcfg.Teams), len(allAgents))

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create runtime adapter registry
	registry := newAdapterRegistry()
	// Create question state stores
	qs := newQuestionStore()
	cas := newCustomAnswerStore()
	initAdaptersMultiTeam(ctx, mcfg, registry, qs)

	// Discover dynamic commands once at startup
	discovered := DiscoverCommands(mcfg.Global.Sync.CommandsPaths)
	allCommands := AllCommands(discovered)
	log.Printf("[daemon] discovered %d dynamic commands", len(discovered))

	// Register Telegram bot commands (best-effort, non-fatal, deduplicate by token)
	registeredBots := make(map[string]bool)
	for _, ta := range allAgents {
		if ta.Config.BotToken == "" || registeredBots[ta.Config.BotToken] {
			continue
		}
		if err := RegisterBotCommands(ta.Config.BotToken, allCommands); err != nil {
			log.Printf("[daemon] warning: failed to register bot commands for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] registered bot commands for %s", ta.AgentName)
		}
		registeredBots[ta.Config.BotToken] = true
	}

	// Deduplicate Telegram pollers by bot token — one poller per unique token
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

	for botToken, targets := range tokenTargets {
		dispatchMap := make(map[int64]pollerTarget)
		for _, t := range targets {
			chatID, err := telegram.ParseChatID(t.chatID)
			if err != nil {
				log.Printf("[daemon] invalid chat_id for %s: %v", t.agentName, err)
				continue
			}
			if existing, ok := dispatchMap[chatID]; ok {
				log.Printf("[daemon] WARNING: chat ID %d collision — agent %s/%s overwrites %s/%s (same bot token, same chat)",
					chatID, t.teamName, t.agentName, existing.teamName, existing.agentName)
			}
			dispatchMap[chatID] = t
		}
		log.Printf("[daemon] starting multi-agent poller for %d agents on token ...%s",
			len(targets), botToken[len(botToken)-min(4, len(botToken)):])
		startMultiAgentPoller(botToken, dispatchMap, func(teamName, agentName, text string) {
			if err := deliverToAgent(registry, teamName, agentName, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", agentName, err)
			}
		}, done, qs, cas, registry, allCommands)
	}

	// Start cleanup watcher for post-merge worker lifecycle
	startCleanupWatcher(done)

	// Start JSONL watcher for CC -> Telegram bridging (skip if all agents are OpenClaw)
	hasNonOpenClaw := false
	for _, ta := range allAgents {
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
		if rt != runtime.OpenClaw {
			hasNonOpenClaw = true
			break
		}
	}
	if hasNonOpenClaw {
		startWatcherMultiTeam(mcfg, qs, done)
	} else {
		log.Printf("[daemon] all agents use OpenClaw — skipping JSONL watcher")
	}

	// Start socket listener
	cleanup, err := listenSocket(sockPath, socketHandlers{
		send: func(req SendRequest) error {
			return handleSendMultiTeam(mcfg, registry, req)
		},
		statusUpdate: handleStatusUpdate,
	})
	if err != nil {
		close(done)
		return err
	}

	// Periodically clean up stale question batches
	go func() {
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
	}()

	log.Printf("[daemon] ready")

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Printf("[daemon] shutting down")
	close(done)
	cancel()
	registry.stopAll(context.Background())
	cleanup()

	return nil
}

// initAdaptersMultiTeam creates and starts runtime adapters for OC/Codex/OpenClaw agents.
// Config-driven: iterates all teams from MultiTeamConfig, no DB required.
func initAdaptersMultiTeam(ctx context.Context, mcfg *config.MultiTeamConfig, registry *adapterRegistry, qs *questionStore) {
	for _, ta := range mcfg.AllAgents() {
		agentPath := filepath.Join(ta.TeamPath, ta.AgentName)

		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)

		// Only register adapters for runtimes the daemon manages.
		// CC agents use tmux directly — registering them would shadow the tmux
		// fallback in deliverToAgent() since SendMessage() requires Start().
		if rt != runtime.OpenCode && rt != runtime.Codex && rt != runtime.OpenClaw {
			continue
		}

		model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
		port := ta.Config.Port
		env := buildAgentEnvMultiTeam(ta.AgentName, ta.TeamName, mcfg)

		team := mcfg.Teams[ta.TeamName]
		adapter := createAdapterFromTeam(ta.AgentName, rt, agentPath, port, model, true, env, team)
		if err := adapter.Start(ctx); err != nil {
			log.Printf("[daemon] failed to start %s adapter for %s: %v", rt, ta.AgentName, err)
			continue
		}
		registry.set(ta.AgentName, adapter)
		log.Printf("[daemon] started %s adapter for %s on port %d", rt, ta.AgentName, port)
		// OpenClaw owns messaging — skip Telegram event bridging
		if rt != runtime.OpenClaw {
			bridgeEventsMultiTeam(ta.AgentName, ta.TeamName, adapter, mcfg, qs)
		}
	}
}

// buildAgentEnvMultiTeam returns env vars for an agent adapter.
func buildAgentEnvMultiTeam(agentName, teamName string, mcfg *config.MultiTeamConfig) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
	}
	if teamName != "default" {
		env = append(env, fmt.Sprintf("TTAL_TEAM=%s", teamName))
	}
	if team, ok := mcfg.Teams[teamName]; ok && team.TaskRC != "" {
		env = append(env, fmt.Sprintf("TASKRC=%s", team.TaskRC))
	}
	return env
}

// bridgeEventsMultiTeam reads events from an adapter and routes them to Telegram.
func bridgeEventsMultiTeam(agentName, teamName string, adapter runtime.Adapter, mcfg *config.MultiTeamConfig, qs *questionStore) {
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
				handleIncomingQuestionMultiTeam(qs, teamName, agentName, adapter.Runtime(), event.CorrelationID, event.Questions, mcfg)
			}
		}
	}()
}

// handleSendMultiTeam routes an incoming SendRequest based on From/To fields.
// Resolves team from agent name or the Team field in the request.
func handleSendMultiTeam(mcfg *config.MultiTeamConfig, registry *adapterRegistry, req SendRequest) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFromMultiTeam(mcfg, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgentMultiTeam(mcfg, registry, req)
	case req.From != "":
		return handleFromMultiTeam(mcfg, req)
	case req.To != "":
		return handleToMultiTeam(mcfg, registry, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleFromMultiTeam sends a message from an agent to the human via Telegram.
func handleFromMultiTeam(mcfg *config.MultiTeamConfig, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.From)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if ta.Config.BotToken == "" {
		return fmt.Errorf("agent %s has no telegram configured", req.From)
	}
	return telegram.SendMessage(ta.Config.BotToken, ta.ChatID, req.Message)
}

// handleToMultiTeam delivers a message to an agent via its runtime adapter.
func handleToMultiTeam(mcfg *config.MultiTeamConfig, registry *adapterRegistry, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	return deliverToAgent(registry, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgentMultiTeam delivers a message from one agent to another.
func handleAgentToAgentMultiTeam(mcfg *config.MultiTeamConfig, registry *adapterRegistry, req SendRequest) error {
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
	return deliverToAgent(registry, toTA.TeamName, req.To, msg)
}

// resolveAgent finds an agent by name, using team hint if provided.
func resolveAgent(mcfg *config.MultiTeamConfig, teamHint, agentName string) *config.TeamAgent {
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
	s := status.AgentStatus{
		Agent:               req.Agent,
		ContextUsedPct:      req.ContextUsedPct,
		ContextRemainingPct: req.ContextRemainingPct,
		ModelID:             req.ModelID,
		SessionID:           req.SessionID,
		UpdatedAt:           time.Now(),
	}
	if err := status.WriteAgent(s); err != nil {
		log.Printf("[daemon] failed to write status for %s: %v", req.Agent, err)
	}
}

// startWatcherMultiTeam initializes the JSONL watcher from config (all teams).
func startWatcherMultiTeam(mcfg *config.MultiTeamConfig, qs *questionStore, done <-chan struct{}) {
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
			if err := telegram.SendMessage(ta.Config.BotToken, ta.ChatID, text); err != nil {
				log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
			}
		},
		func(teamName, agentName, correlationID string, questions []runtime.Question) {
			handleIncomingQuestionMultiTeam(qs, teamName, agentName, runtime.ClaudeCode, correlationID, questions, mcfg)
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

// handleIncomingQuestionMultiTeam handles questions with team context.
func handleIncomingQuestionMultiTeam(
	store *questionStore,
	teamName, agentName string,
	rt runtime.Runtime,
	correlationID string,
	questions []runtime.Question,
	mcfg *config.MultiTeamConfig,
) {
	if len(questions) == 0 {
		return
	}

	for _, q := range questions {
		if q.MultiSelect {
			log.Printf("[questions] warning: multi-select not supported in Telegram UI for %s question %q — treating as single-select", agentName, q.Header)
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

func writePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

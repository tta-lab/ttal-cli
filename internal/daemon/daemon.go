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

	"codeberg.org/clawteam/ttal-cli/ent"
	entagent "codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/status"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"codeberg.org/clawteam/ttal-cli/internal/watcher"
)

const pidFileName = "daemon.pid"

// Run starts the daemon in the foreground. This is what launchd calls.
func Run(database *ent.Client) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// Refuse to start if already running
	if running, pid, _ := IsRunning(); running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}

	dataDir := cfg.DataDir()
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

	log.Printf("[daemon] starting — socket=%s agents=%d", sockPath, len(cfg.Agents))

	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create runtime adapter registry
	registry := newAdapterRegistry()
	// Create question state stores
	qs := newQuestionStore()
	cas := newCustomAnswerStore()
	initAdapters(ctx, database, cfg, registry, qs)

	// Discover dynamic commands once at startup
	discovered := DiscoverCommands(cfg.Sync.CommandsPaths)
	allCommands := AllCommands(discovered)
	log.Printf("[daemon] discovered %d dynamic commands", len(discovered))

	// Register Telegram bot commands (best-effort, non-fatal)
	registeredBots := make(map[string]bool)
	for name, agentCfg := range cfg.Agents {
		if agentCfg.BotToken == "" || registeredBots[agentCfg.BotToken] {
			continue
		}
		if err := RegisterBotCommands(agentCfg.BotToken, allCommands); err != nil {
			log.Printf("[daemon] warning: failed to register bot commands for %s: %v", name, err)
		} else {
			log.Printf("[daemon] registered bot commands for %s", name)
		}
		registeredBots[agentCfg.BotToken] = true
	}

	// Start Telegram pollers (skip for OpenClaw agents — OpenClaw owns messaging)
	for agentName, agentCfg := range cfg.Agents {
		if agentCfg.BotToken == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", agentName)
			continue
		}
		rt := resolveAgentRuntime(ctx, database, cfg, agentName)
		if rt == runtime.OpenClaw {
			log.Printf("[daemon] agent %s uses OpenClaw — skipping Telegram poller", agentName)
			continue
		}
		log.Printf("[daemon] starting telegram poller for %s", agentName)
		teamName := cfg.TeamName()
		startTelegramPoller(teamName, agentName, agentCfg, cfg.AgentChatID(agentName), func(name, text string) {
			if err := deliverToAgent(registry, teamName, name, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", name, err)
			}
		}, done, qs, cas, registry, allCommands)
	}

	// Start cleanup watcher for post-merge worker lifecycle
	startCleanupWatcher(done)

	// Start JSONL watcher for CC -> Telegram bridging (skip if all agents are OpenClaw)
	hasNonOpenClaw := false
	for agentName := range cfg.Agents {
		rt := resolveAgentRuntime(ctx, database, cfg, agentName)
		if rt != runtime.OpenClaw {
			hasNonOpenClaw = true
			break
		}
	}
	if hasNonOpenClaw {
		startWatcher(database, cfg, qs, done)
	} else {
		log.Printf("[daemon] all agents use OpenClaw — skipping JSONL watcher")
	}

	// Start socket listener
	cleanup, err := listenSocket(sockPath, socketHandlers{
		send: func(req SendRequest) error {
			return handleSend(cfg, registry, req)
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

// initAdapters creates and starts runtime adapters for OC/Codex agents.
// CC agents use the existing JSONL watcher path — their adapters are only
// registered but not started from the daemon (team start handles tmux).
func initAdapters(ctx context.Context, database *ent.Client, cfg *config.Config, registry *adapterRegistry, qs *questionStore) {
	for agentName := range cfg.Agents {
		agentPath := cfg.AgentPath(agentName)
		if agentPath == "" {
			log.Printf("[daemon] agent %q has no path (set team_path in config), skipping", agentName)
			continue
		}

		ag, err := database.Agent.Query().Where(entagent.Name(agentName)).Only(ctx)
		if err != nil {
			log.Printf("[daemon] skipping adapter for %s: db lookup failed: %v", agentName, err)
			continue
		}

		rt := cfg.AgentRuntime()
		if ag.Runtime != nil {
			rt = runtime.Runtime(*ag.Runtime)
		}

		// Only register adapters for runtimes the daemon manages.
		// CC agents use tmux directly — registering them would shadow the tmux
		// fallback in deliverToAgent() since SendMessage() requires Start().
		if rt != runtime.OpenCode && rt != runtime.Codex && rt != runtime.OpenClaw {
			continue
		}

		port := cfg.Agents[agentName].Port
		env := buildAgentEnv(agentName, cfg)

		adapter := createAdapter(agentName, rt, agentPath, port, string(ag.Model), true, env, cfg)
		if err := adapter.Start(ctx); err != nil {
			log.Printf("[daemon] failed to start %s adapter for %s: %v", rt, agentName, err)
			continue
		}
		registry.set(agentName, adapter)
		log.Printf("[daemon] started %s adapter for %s on port %d", rt, agentName, port)
		// OpenClaw owns messaging — skip Telegram event bridging
		if rt != runtime.OpenClaw {
			bridgeEvents(agentName, adapter, cfg, qs)
		}
	}
}

// resolveAgentRuntime returns the effective runtime for an agent,
// checking the per-agent DB override first, then the team agent_runtime.
func resolveAgentRuntime(
	ctx context.Context, database *ent.Client, cfg *config.Config, agentName string,
) runtime.Runtime {
	rt := cfg.AgentRuntime()
	if ag, err := database.Agent.Query().Where(entagent.Name(agentName)).Only(ctx); err == nil && ag.Runtime != nil {
		rt = runtime.Runtime(*ag.Runtime)
	}
	return rt
}

// buildAgentEnv returns env vars for an agent adapter.
func buildAgentEnv(agentName string, cfg *config.Config) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
	}
	if team := cfg.TeamName(); team != "default" || os.Getenv("TTAL_TEAM") != "" {
		env = append(env, fmt.Sprintf("TTAL_TEAM=%s", team))
	}
	if taskrc := cfg.TaskRC(); taskrc != config.DefaultTaskRC() {
		env = append(env, fmt.Sprintf("TASKRC=%s", taskrc))
	}
	return env
}

// handleSend routes an incoming SendRequest based on From/To fields.
// Special target "human" routes to Telegram instead of agent delivery.
func handleSend(cfg *config.Config, registry *adapterRegistry, req SendRequest) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(cfg, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgent(cfg, registry, req)
	case req.From != "":
		return handleFrom(cfg, req)
	case req.To != "":
		return handleTo(cfg, registry, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleFrom sends a message from an agent to the human via Telegram.
func handleFrom(cfg *config.Config, req SendRequest) error {
	agentCfg, ok := cfg.Agents[req.From]
	if !ok {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if agentCfg.BotToken == "" {
		return fmt.Errorf("agent %s has no telegram configured", req.From)
	}
	return telegram.SendMessage(agentCfg.BotToken, cfg.AgentChatID(req.From), req.Message)
}

// handleTo delivers a message to an agent via its runtime adapter.
func handleTo(cfg *config.Config, registry *adapterRegistry, req SendRequest) error {
	if _, ok := cfg.Agents[req.To]; !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	return deliverToAgent(registry, cfg.TeamName(), req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another,
// wrapping the message with attribution so the recipient knows who sent it.
func handleAgentToAgent(cfg *config.Config, registry *adapterRegistry, req SendRequest) error {
	if _, ok := cfg.Agents[req.From]; !ok {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if _, ok := cfg.Agents[req.To]; !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	msg := formatAgentMessage(req.From, req.Message)
	log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
	return deliverToAgent(registry, cfg.TeamName(), req.To, msg)
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

// startWatcher initializes and runs the JSONL watcher in a goroutine.
func startWatcher(database *ent.Client, cfg *config.Config, qs *questionStore, done <-chan struct{}) {
	w, err := watcher.New(database,
		func(agentName, text string) {
			agentCfg, ok := cfg.Agents[agentName]
			if !ok || agentCfg.BotToken == "" {
				return
			}
			if err := telegram.SendMessage(agentCfg.BotToken, cfg.AgentChatID(agentName), text); err != nil {
				log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
			}
		},
		func(agentName, correlationID string, questions []runtime.Question) {
			handleIncomingQuestion(qs, agentName, runtime.ClaudeCode, correlationID, questions, cfg)
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

// IsRunning checks whether the daemon is running by inspecting the pid file.
func IsRunning() (bool, int, error) {
	pidPath := filepath.Join(config.ResolveDataDir(), pidFileName)
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

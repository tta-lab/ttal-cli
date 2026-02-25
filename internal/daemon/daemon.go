package daemon

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/internal/config"
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

	// Register Telegram bot commands (best-effort, non-fatal)
	registeredBots := make(map[string]bool)
	for name, agentCfg := range cfg.Agents {
		if agentCfg.BotToken == "" || registeredBots[agentCfg.BotToken] {
			continue
		}
		if err := RegisterBotCommands(agentCfg.BotToken); err != nil {
			log.Printf("[daemon] warning: failed to register bot commands for %s: %v", name, err)
		} else {
			log.Printf("[daemon] registered bot commands for %s", name)
		}
		registeredBots[agentCfg.BotToken] = true
	}

	// Start Telegram pollers
	for agentName, agentCfg := range cfg.Agents {
		if agentCfg.BotToken == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", agentName)
			continue
		}
		log.Printf("[daemon] starting telegram poller for %s", agentName)
		startTelegramPoller(agentName, agentCfg, cfg.AgentChatID(agentName), func(name, text string) {
			if err := deliverToAgent(name, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", name, err)
			}
		}, done)
	}

	// Start cleanup watcher for post-merge worker lifecycle
	startCleanupWatcher(done)

	// Start JSONL watcher for CC -> Telegram bridging
	startWatcher(database, cfg, done)

	// Start socket listener
	cleanup, err := listenSocket(sockPath, socketHandlers{
		send: func(req SendRequest) error {
			return handleSend(cfg, req)
		},
		statusUpdate: handleStatusUpdate,
	})
	if err != nil {
		close(done)
		return err
	}

	log.Printf("[daemon] ready")

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Printf("[daemon] shutting down")
	close(done)
	cleanup()

	return nil
}

// handleSend routes an incoming SendRequest based on From/To fields.
// Special target "human" routes to Telegram instead of agent delivery.
func handleSend(cfg *config.Config, req SendRequest) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(cfg, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgent(cfg, req)
	case req.From != "":
		return handleFrom(cfg, req)
	case req.To != "":
		return handleTo(cfg, req)
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

// handleTo delivers a message to an agent's tmux session.
func handleTo(cfg *config.Config, req SendRequest) error {
	if _, ok := cfg.Agents[req.To]; !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	return deliverToAgent(req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another via tmux,
// wrapping the message with attribution so the recipient knows who sent it.
func handleAgentToAgent(cfg *config.Config, req SendRequest) error {
	if _, ok := cfg.Agents[req.From]; !ok {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if _, ok := cfg.Agents[req.To]; !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	msg := formatAgentMessage(req.From, req.Message)
	log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
	return deliverToAgent(req.To, msg)
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
func startWatcher(database *ent.Client, cfg *config.Config, done <-chan struct{}) {
	w, err := watcher.New(database, func(agentName, text string) {
		agentCfg, ok := cfg.Agents[agentName]
		if !ok || agentCfg.BotToken == "" {
			return
		}
		if err := telegram.SendMessage(agentCfg.BotToken, cfg.AgentChatID(agentName), text); err != nil {
			log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
		}
	})
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

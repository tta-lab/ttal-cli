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

	"codeberg.org/clawteam/ttal-cli/internal/worker"
)

const (
	pidFileName  = "daemon.pid"
	pollInterval = 60 * time.Second
)

// Run starts the daemon in the foreground. This is what launchd calls.
func Run() error {
	cfg, err := LoadConfig()
	if err != nil {
		return err
	}

	// Refuse to start if already running
	if running, pid, _ := IsRunning(); running {
		return fmt.Errorf("daemon already running (pid=%d)", pid)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	ttalDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		return err
	}

	// Write pid file
	pidPath := filepath.Join(ttalDir, pidFileName)
	if err := writePID(pidPath); err != nil {
		return fmt.Errorf("failed to write pid file: %w", err)
	}
	defer os.Remove(pidPath) //nolint:errcheck

	sockPath, err := SocketPath()
	if err != nil {
		return err
	}

	log.Printf("[daemon] starting — socket=%s agents=%d", sockPath, len(cfg.Agents))

	done := make(chan struct{})

	// Start Telegram pollers
	for agentName, agentCfg := range cfg.Agents {
		if agentCfg.Telegram.BotToken == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", agentName)
			continue
		}
		log.Printf("[daemon] starting telegram poller for %s", agentName)
		startTelegramPoller(agentName, agentCfg, func(name, text string) {
			ac := cfg.Agents[name]
			if err := deliverToZellij(ac.Zellij, text); err != nil {
				log.Printf("[daemon] zellij delivery failed for %s: %v", name, err)
			}
		}, done)
	}

	// Start completion poller
	go runCompletionPoller(done)

	// Start socket listener
	cleanup, err := listenSocket(sockPath, func(req SendRequest) error {
		return handleSend(cfg, req)
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
func handleSend(cfg *Config, req SendRequest) error {
	switch {
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
func handleFrom(cfg *Config, req SendRequest) error {
	agentCfg, ok := cfg.Agents[req.From]
	if !ok {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	if agentCfg.Telegram.BotToken == "" {
		return fmt.Errorf("agent %s has no telegram configured", req.From)
	}
	return sendTelegramMessage(agentCfg.Telegram.BotToken, agentCfg.Telegram.ChatID, req.Message)
}

// handleTo delivers a message to an agent's zellij session.
func handleTo(cfg *Config, req SendRequest) error {
	agentCfg, ok := cfg.Agents[req.To]
	if !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	return deliverToZellij(agentCfg.Zellij, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another via zellij,
// wrapping the message with attribution so the recipient knows who sent it.
func handleAgentToAgent(cfg *Config, req SendRequest) error {
	if _, ok := cfg.Agents[req.From]; !ok {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	toAgentCfg, ok := cfg.Agents[req.To]
	if !ok {
		return fmt.Errorf("unknown agent: %s", req.To)
	}
	msg := formatAgentMessage(req.From, req.Message)
	log.Printf("[daemon] agent-to-agent: %s → %s", req.From, req.To)
	return deliverToZellij(toAgentCfg.Zellij, msg)
}

// runCompletionPoller runs worker.Poll every 60s until done is closed.
func runCompletionPoller(done <-chan struct{}) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Run once immediately
	if err := worker.Poll(); err != nil {
		log.Printf("[daemon] completion poll error: %v", err)
	}

	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			if err := worker.Poll(); err != nil {
				log.Printf("[daemon] completion poll error: %v", err)
			}
		}
	}
}

// IsRunning checks whether the daemon is running by inspecting the pid file.
func IsRunning() (bool, int, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, 0, err
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

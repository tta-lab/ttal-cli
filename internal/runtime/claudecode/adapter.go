package claudecode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// Adapter communicates with Claude Code via tmux sessions and JSONL file watching.
type Adapter struct {
	cfg       runtime.AdapterConfig
	events    chan runtime.Event
	sessionID string // tmux session name
	watcher   *watcher.AgentWatcher
	done      chan struct{}
	wg        sync.WaitGroup
}

// New creates a CC adapter.
func New(cfg runtime.AdapterConfig) *Adapter {
	return &Adapter{
		cfg:    cfg,
		events: make(chan runtime.Event, 64),
		done:   make(chan struct{}),
	}
}

func (a *Adapter) Runtime() runtime.Runtime { return runtime.ClaudeCode }

func (a *Adapter) Start(_ context.Context) error {
	shellCfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	sessionName := config.AgentSessionName(shellCfg.TeamName(), a.cfg.AgentName)

	cmd := "claude --dangerously-skip-permissions"
	if a.cfg.Model != "" {
		cmd += " --model " + a.cfg.Model
	}
	if hasConversation(a.cfg.WorkDir) {
		cmd += " --continue"
	}

	shellCmd := shellCfg.BuildEnvShellCommand(a.cfg.Env, cmd)

	if err := tmux.NewSession(sessionName, a.cfg.AgentName, a.cfg.WorkDir, shellCmd); err != nil {
		return fmt.Errorf("create tmux session: %w", err)
	}

	for _, kv := range a.cfg.Env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			_ = tmux.SetEnv(sessionName, parts[0], parts[1])
		}
	}

	a.sessionID = sessionName

	a.watcher = watcher.NewAgentWatcher(a.cfg.AgentName, a.cfg.WorkDir, func(text string) {
		select {
		case <-a.done:
		case a.events <- runtime.Event{
			Type:  runtime.EventText,
			Agent: a.cfg.AgentName,
			Text:  text,
		}:
		}
	})
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		a.watcher.Run(a.done)
	}()

	return nil
}

func (a *Adapter) Stop(_ context.Context) error {
	close(a.done)
	a.wg.Wait()
	if a.sessionID != "" {
		_ = tmux.KillSession(a.sessionID)
	}
	close(a.events)
	return nil
}

func (a *Adapter) SendMessage(_ context.Context, text string) error {
	if a.sessionID == "" {
		return fmt.Errorf("CC adapter not started")
	}
	return tmux.SendKeys(a.sessionID, a.cfg.AgentName, text)
}

func (a *Adapter) Events() <-chan runtime.Event {
	return a.events
}

func (a *Adapter) CreateSession(_ context.Context) (string, error) {
	return "", nil
}

func (a *Adapter) ResumeSession(_ context.Context, _ string) (string, error) {
	return "", nil
}

func (a *Adapter) IsHealthy(_ context.Context) bool {
	if a.sessionID == "" {
		return false
	}
	return tmux.SessionExists(a.sessionID)
}

// hasConversation checks for existing CC session JSONL files.
func hasConversation(workDir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	encoded := strings.ReplaceAll(workDir, string(filepath.Separator), "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	projectDir := filepath.Join(home, ".claude", "projects", encoded)
	matches, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	return len(matches) > 0
}

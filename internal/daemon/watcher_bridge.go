package daemon

import (
	"context"
	"log"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// startWatcher initializes the JSONL watcher from config (all teams).
func startWatcher(
	cfg *config.Config, frontends map[string]frontend.Frontend, msgSvc *message.Service, done <-chan struct{},
) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] watcher disabled: cannot get home directory: %v — CC→Telegram bridging will not work", err)
		return
	}
	defaultProjectsDir := filepath.Join(home, ".claude", "projects")

	agentMap := make(map[string]watcher.WatchedAgent)
	for _, ta := range cfg.Agents() {
		encoded := watcher.EncodePath(filepath.Join(ta.TeamPath, ta.AgentName))
		projectsDir := defaultProjectsDir

		// Composite key avoids collision when multiple teams have same agent name
		key := "default" + "/" + encoded
		agentMap[key] = watcher.WatchedAgent{
			AgentInfo:   watcher.AgentInfo{TeamName: "default", AgentName: ta.AgentName},
			ProjectsDir: projectsDir,
			EncodedDir:  encoded,
		}
	}

	w, err := watcher.New(agentMap,
		func(teamName, agentName, text string) {
			_, ok := cfg.FindAgent(agentName)
			if !ok {
				return
			}
			fe, ok := frontends["default"]
			if !ok {
				return
			}
			rt := cfg.RuntimeForAgent(agentName)
			persistMsg(msgSvc, message.CreateParams{
				Sender: agentName, Recipient: cfg.UserName, Content: text,
				Team: "default", Channel: message.ChannelWatcher, Runtime: &rt,
			})
			if err := fe.SendText(context.Background(), agentName, text); err != nil {
				log.Printf("[watcher] send error for %s/%s: %v", "default", agentName, err)
			} else {
				_ = fe.ClearTracking(context.Background(), agentName)
			}
		},
		func(teamName, agentName, toolName string) {
			emoji := telegram.ToolEmoji(toolName)
			if emoji == "" {
				return
			}
			if cfg.TeamPath == "" || !cfg.EmojiReactions {
				return
			}
			fe, ok := frontends["default"]
			if !ok {
				return
			}
			if err := fe.SetReaction(context.Background(), agentName, emoji); err != nil {
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

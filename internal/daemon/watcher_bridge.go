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
	mcfg *config.DaemonConfig, frontends map[string]frontend.Frontend, msgSvc *message.Service, done <-chan struct{},
) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] watcher disabled: cannot get home directory: %v — CC→Telegram bridging will not work", err)
		return
	}
	defaultProjectsDir := filepath.Join(home, ".claude", "projects")

	agentMap := make(map[string]watcher.WatchedAgent)
	for _, ta := range mcfg.AllAgents() {
		encoded := watcher.EncodePath(filepath.Join(ta.TeamPath, ta.AgentName))
		projectsDir := defaultProjectsDir

		// Composite key avoids collision when multiple teams have same agent name
		key := ta.TeamName + "/" + encoded
		agentMap[key] = watcher.WatchedAgent{
			AgentInfo:   watcher.AgentInfo{TeamName: ta.TeamName, AgentName: ta.AgentName},
			ProjectsDir: projectsDir,
			EncodedDir:  encoded,
		}
	}

	w, err := watcher.New(agentMap,
		func(teamName, agentName, text string) {
			ta, ok := mcfg.FindAgentInTeam(teamName, agentName)
			if !ok {
				return
			}
			fe, ok := frontends[teamName]
			if !ok {
				return
			}
			rt := mcfg.AgentRuntimeForTeam(teamName, ta.TeamPath, agentName)
			persistMsg(msgSvc, message.CreateParams{
				Sender: agentName, Recipient: mcfg.Global.UserName(), Content: text,
				Team: teamName, Channel: message.ChannelWatcher, Runtime: &rt,
			})
			if err := fe.SendText(context.Background(), agentName, text); err != nil {
				log.Printf("[watcher] send error for %s/%s: %v", teamName, agentName, err)
			} else {
				_ = fe.ClearTracking(context.Background(), agentName)
			}
		},
		func(teamName, agentName, toolName string) {
			emoji := telegram.ToolEmoji(toolName)
			if emoji == "" {
				return
			}
			// Check if emoji reactions are enabled for this team
			if team, ok := mcfg.Teams[teamName]; !ok || !team.EmojiReactions {
				return
			}
			fe, ok := frontends[teamName]
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

package daemon

import (
	"log"
	"os"
	"path/filepath"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// startWatcher initializes the JSONL watcher from config (all teams).
func startWatcher(
	mcfg *config.DaemonConfig, mt *messageTracker, msgSvc *message.Service, done <-chan struct{},
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
			botToken := config.AgentBotToken(agentName)
			if !ok || botToken == "" {
				return
			}
			// Clear tracking — response text arriving is the done signal
			mt.delete(teamName, agentName)
			rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
			persistMsg(msgSvc, message.CreateParams{
				Sender: agentName, Recipient: mcfg.Global.UserName(), Content: text,
				Team: teamName, Channel: message.ChannelWatcher, Runtime: &rt,
			})
			if err := telegram.SendMessage(botToken, ta.ChatID, text); err != nil {
				log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
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

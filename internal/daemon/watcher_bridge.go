package daemon

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

var (
	taskScopedWatchers   = make(map[string]chan struct{}) // teamName/agentName → done channel
	taskScopedWatchersMu sync.Mutex
)

// onTaskScopedSpawn is called after a task-scoped session is created.
// Set by daemon.go after frontends are built. Nil = warn + skip.
var onTaskScopedSpawn func(teamName, agentName, sessionID, workDir string)

// startTaskScopedFileWatch starts a per-file JSONL watcher for a task-scoped agent session.
// It stops any existing watcher for the same agent before starting the new one.
func startTaskScopedFileWatch(
	mcfg *config.DaemonConfig,
	frontends map[string]frontend.Frontend,
	msgSvc *message.Service,
	teamName, agentName, sessionID, workDir string,
) {
	watcherKey := teamName + "/" + agentName
	taskScopedWatchersMu.Lock()
	if oldDone, ok := taskScopedWatchers[watcherKey]; ok {
		close(oldDone)
		delete(taskScopedWatchers, watcherKey)
	}
	taskScopedWatchersMu.Unlock()

	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[watcher] task-scoped: cannot get home dir for %s: %v", agentName, err)
		return
	}
	encoded := watcher.EncodePath(workDir)
	jsonlPath := filepath.Join(home, ".claude", "projects", encoded, sessionID+".jsonl")

	done := make(chan struct{})
	fw := watcher.NewFileWatcher(agentName, jsonlPath, func(text string) {
		fe, ok := frontends[teamName]
		if !ok {
			return
		}
		rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
		persistMsg(msgSvc, message.CreateParams{
			Sender:    agentName,
			Recipient: mcfg.Global.UserName(),
			Content:   text,
			Team:      teamName,
			Channel:   message.ChannelWatcher,
			Runtime:   &rt,
		})
		if err := fe.SendText(context.Background(), agentName, text); err != nil {
			log.Printf("[watcher] task-scoped send error for %s: %v", agentName, err)
		} else {
			_ = fe.ClearTracking(context.Background(), agentName)
		}
	})

	taskScopedWatchersMu.Lock()
	taskScopedWatchers[watcherKey] = done
	taskScopedWatchersMu.Unlock()

	go fw.Run(done)
	log.Printf("[watcher] task-scoped file watch started for %s (%s)", agentName, jsonlPath)
}

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
			if _, ok := mcfg.FindAgentInTeam(teamName, agentName); !ok {
				return
			}
			fe, ok := frontends[teamName]
			if !ok {
				return
			}
			rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
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

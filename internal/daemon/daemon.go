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

	entsql "entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/watcher"

	_ "modernc.org/sqlite"
)

const pidFileName = "daemon.pid"

// k8sPods maps team name → running k8s team pod.
// Populated during daemon startup (Phase 1 of initAdapters) and used by deliverToAgent.
// Written once at startup before pollers start — no mutex needed.
var k8sPods map[string]*k8sTeamPod

// pollerTarget groups agent info for Telegram poller dispatch by chat ID.
type pollerTarget struct {
	teamName  string
	agentName string
	chatID    string
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

	// Open SQLite message database.
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}
	ttalDir := filepath.Join(home, ".ttal")
	if err := os.MkdirAll(ttalDir, 0o755); err != nil {
		return fmt.Errorf("create message db dir: %w", err)
	}
	dbPath := filepath.Join(ttalDir, "messages.db")
	// modernc/sqlite uses _pragma= syntax; foreign_keys and WAL mode required.
	dbDSN := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	drv, err := entsql.Open("sqlite", dbDSN)
	if err != nil {
		return fmt.Errorf("open message db: %w", err)
	}
	entClient := ent.NewClient(ent.Driver(entsql.OpenDB("sqlite3", drv.DB())))
	defer func() { _ = entClient.Close() }()
	if err := entClient.Schema.Create(context.Background()); err != nil {
		return fmt.Errorf("migrate message schema: %w", err)
	}
	msgSvc := message.NewService(entClient)

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
		initAdapters(ctx, mcfg, registry, qs, mt, msgSvc)
	}()

	startupWg.Add(1)
	go func() {
		defer startupWg.Done()
		allCommands = discoverAndRegisterCommands(mcfg, allAgents)
	}()

	startupWg.Wait()
	startTelegramPollers(mcfg, allAgents, registry, done, qs, cas, allCommands, mt, msgSvc)
	startNotificationPollers(mcfg, done)
	startUsagePoller(done)
	startHeartbeatScheduler(mcfg, registry, done)
	startCleanupWatcher(done)
	startPRWatcher(mcfg, done)
	startReminderPoller(mcfg, done)
	startWatcherIfNeeded(mcfg, qs, mt, msgSvc, done)

	cleanup, err := listenSocket(sockPath, socketHandlers{
		send: func(req SendRequest) error {
			return handleSend(mcfg, registry, msgSvc, req)
		},
		statusUpdate: handleStatusUpdate,
		taskComplete: func(raw []byte) SendResponse {
			return handleTaskComplete(raw, mcfg, registry)
		},
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
		token := config.AgentBotToken(ta.AgentName)
		if token == "" {
			continue
		}
		if _, ok := tokenAgent[token]; !ok {
			tokenAgent[token] = ta.AgentName
		}
	}
	// Include notification bot tokens so they also get command menus.
	for teamName, team := range mcfg.Teams {
		if team.NotificationToken == "" {
			continue
		}
		if _, ok := tokenAgent[team.NotificationToken]; !ok {
			tokenAgent[team.NotificationToken] = teamName + "-notify"
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
	mt *messageTracker, msgSvc *message.Service,
) {
	tokenTargets := buildTokenTargets(allAgents)

	for botToken, targets := range tokenTargets {
		dispatchMap := buildDispatchMap(targets)
		log.Printf("[daemon] starting multi-agent poller for %d agents on token ...%s",
			len(targets), botToken[len(botToken)-min(4, len(botToken)):])
		startMultiAgentPoller(botToken, dispatchMap, func(teamName, agentName, text string) {
			if err := deliverToAgent(registry, mcfg, teamName, agentName, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", agentName, err)
			}
		}, done, qs, cas, allCommands, mt, msgSvc,
			func(teamName string) string { return mcfg.UserNameForTeam(teamName) })
	}
}

// buildTokenTargets groups agents by bot token, skipping those without tokens.
func buildTokenTargets(allAgents []config.TeamAgent) map[string][]pollerTarget {
	tokenTargets := make(map[string][]pollerTarget)
	for _, ta := range allAgents {
		token := config.AgentBotToken(ta.AgentName)
		if token == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", ta.AgentName)
			continue
		}
		tokenTargets[token] = append(tokenTargets[token], pollerTarget{
			teamName:  ta.TeamName,
			agentName: ta.AgentName,
			chatID:    ta.ChatID,
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

// startWatcherIfNeeded starts the JSONL watcher.
func startWatcherIfNeeded(
	mcfg *config.DaemonConfig,
	qs *questionStore, mt *messageTracker, msgSvc *message.Service, done <-chan struct{},
) {
	startWatcher(mcfg, qs, mt, msgSvc, done)
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

	s := <-sig
	log.Printf("[daemon] received signal %v — shutting down", s)
	close(done)
	cancel()
	shutdownAgents(mcfg, registry)
	cleanup()
}

// persistMsg persists a message and logs a warning if it fails.
func persistMsg(msgSvc *message.Service, p message.CreateParams) {
	if _, err := msgSvc.Create(context.Background(), p); err != nil {
		log.Printf("[daemon] message persist failed (sender=%s): %v", p.Sender, err)
	}
}

// initAdapters starts all agent sessions in parallel: tmux for CC, HTTP adapters for all others.
// Config-driven: iterates all teams, no DB required.
//
// Two phases:
//  1. Set up k8s team pods (synchronous — pods must be ready before agents spawn).
//  2. Spawn per-agent sessions in parallel (local tmux or k8s exec).
func initAdapters(
	ctx context.Context, mcfg *config.DaemonConfig,
	registry *adapterRegistry, qs *questionStore, mt *messageTracker, msgSvc *message.Service,
) {
	// Phase 1: ensure k8s team pods exist with correct spec
	k8sPods = make(map[string]*k8sTeamPod)
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[k8s] cannot get home directory — skipping all k8s teams: %v", err)
	} else {
		for teamName, team := range mcfg.Teams {
			if !team.IsK8s() {
				continue
			}
			pod := &k8sTeamPod{
				kubectx:   team.Kubernetes.Context,
				namespace: "ttal",
				image:     team.K8sAgentImage(),
				teamName:  teamName,
			}
			if err := pod.EnsureNamespace(); err != nil {
				log.Printf("[k8s] failed to ensure namespace for team %s: %v", teamName, err)
				continue
			}
			if err := bootstrapClaudeDir(teamName, team.TeamPath, home); err != nil {
				log.Printf("[k8s] failed to bootstrap .claude for team %s: %v", teamName, err)
				continue
			}
			sharedEnv, err := buildSharedEnv(teamName)
			if err != nil {
				log.Printf("[k8s] failed to build shared env for team %s: %v", teamName, err)
				continue
			}
			volumes := buildVolumes(team, teamName, home)
			if err := pod.EnsurePod(sharedEnv, volumes); err != nil {
				log.Printf("[k8s] failed to ensure pod for team %s: %v", teamName, err)
				continue
			}
			k8sPods[teamName] = pod
		}
	}

	ensureLocalAgentTrust(mcfg)

	// Phase 2: spawn per-agent sessions in parallel
	var wg sync.WaitGroup
	for _, ta := range mcfg.AllAgents() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			initSingleAdapter(ctx, ta, mcfg, registry, qs, mt, msgSvc)
		}()
	}
	wg.Wait()
}

// initSingleAdapter initializes a single agent: tmux session for CC, HTTP adapter for others.
func initSingleAdapter(
	ctx context.Context, ta config.TeamAgent, mcfg *config.DaemonConfig,
	registry *adapterRegistry, qs *questionStore, mt *messageTracker, msgSvc *message.Service,
) {
	agentPath := filepath.Join(ta.TeamPath, ta.AgentName)

	// K8s agents — spawn tmux session inside team pod instead of local tmux.
	if team, ok := mcfg.Teams[ta.TeamName]; ok && team.IsK8s() {
		pod, ok := k8sPods[ta.TeamName]
		if !ok {
			log.Printf("[daemon] k8s pod not ready for team %s, skipping agent %s", ta.TeamName, ta.AgentName)
			return
		}
		if pod.SessionExists(ta.AgentName) {
			log.Printf("[daemon] k8s agent %s already running in pod ttal-%s", ta.AgentName, ta.TeamName)
			return
		}
		model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
		perAgentEnv := buildPerAgentEnv(ta.AgentName, ta.TeamName, mcfg)
		if err := pod.SpawnAgent(ta.AgentName, model, perAgentEnv); err != nil {
			log.Printf("[daemon] failed to spawn k8s agent %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] k8s agent %s running in pod ttal-%s", ta.AgentName, ta.TeamName)
		}
		return
	}

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
		shell := mcfg.Global.GetShell()
		ensureProjectDir(agentPath)
		if err := spawnCCSession(sessionName, ta.AgentName, agentPath, model, ta.TeamName, env, shell); err != nil {
			log.Printf("[daemon] failed to start CC session for %s: %v", ta.AgentName, err)
		} else {
			log.Printf("[daemon] CC agent %s running (session: %s)", ta.AgentName, sessionName)
		}
		return
	}

	model := mcfg.AgentModelForTeam(ta.TeamName, ta.AgentName)
	env := buildAgentEnv(ta.AgentName, ta.TeamName, mcfg)

	adapter := createAdapterFromTeam(ta.AgentName, agentPath, model, env)
	if err := adapter.Start(ctx); err != nil {
		log.Printf("[daemon] failed to start %s adapter for %s: %v", rt, ta.AgentName, err)
		return
	}
	registry.set(ta.TeamName, ta.AgentName, adapter)
	log.Printf("[daemon] started %s adapter for %s", rt, ta.AgentName)
	bridgeEvents(ta.AgentName, ta.TeamName, adapter, mcfg, qs, mt, msgSvc)
}

// buildAgentEnv returns env vars for an agent adapter.
func buildAgentEnv(agentName, teamName string, mcfg *config.DaemonConfig) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
		fmt.Sprintf("TTAL_TEAM=%s", teamName),
	}
	if team, ok := mcfg.Teams[teamName]; ok && team.TaskRC != "" {
		env = append(env, fmt.Sprintf("TASKRC=%s", team.TaskRC))
	}
	// Read flicknote_project from CLAUDE.md frontmatter
	if team, ok := mcfg.Teams[teamName]; ok && team.TeamPath != "" {
		info, err := agentfs.GetFromPath(filepath.Join(team.TeamPath, agentName))
		if err == nil && info.FlicknoteProject != "" {
			env = append(env, fmt.Sprintf("FLICKNOTE_PROJECT=%s", info.FlicknoteProject))
		}
	}

	// Inject all secrets from .env
	env = append(env, config.DotEnvParts()...)

	return env
}

// buildSharedEnv returns container-level env vars for a k8s team pod.
// Includes TTAL_TEAM and all .env secrets. Does NOT include per-agent vars.
// Returns an error if .env exists but cannot be loaded — a pod created with
// stripped secrets would be reused indefinitely due to spec-hash matching.
func buildSharedEnv(teamName string) ([]string, error) {
	dotenvMap, err := config.LoadDotEnv()
	if err != nil {
		return nil, fmt.Errorf("loading .env for k8s pod: %w", err)
	}
	env := make([]string, 0, 1+len(dotenvMap))
	env = append(env, fmt.Sprintf("TTAL_TEAM=%s", teamName))
	for k, v := range dotenvMap {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env, nil
}

// buildPerAgentEnv returns tmux-session-level env vars for a k8s agent.
// These are passed as `env KEY=VAL` prefix in the tmux command inside the pod.
func buildPerAgentEnv(agentName, teamName string, mcfg *config.DaemonConfig) []string {
	env := []string{
		fmt.Sprintf("TTAL_AGENT_NAME=%s", agentName),
	}
	if team, ok := mcfg.Teams[teamName]; ok && team.TeamPath != "" {
		info, err := agentfs.GetFromPath(filepath.Join(team.TeamPath, agentName))
		if err == nil && info.FlicknoteProject != "" {
			env = append(env, fmt.Sprintf("FLICKNOTE_PROJECT=%s", info.FlicknoteProject))
		}
	}
	return env
}

// ensureLocalAgentTrust adds hasTrustDialogAccepted entries to ~/.claude.json
// for all non-k8s agent workspace paths. Idempotent.
func ensureLocalAgentTrust(mcfg *config.DaemonConfig) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] warning: cannot get home dir for local agent trust: %v", err)
		return
	}

	var paths []string
	for _, ta := range mcfg.AllAgents() {
		team := mcfg.Teams[ta.TeamName]
		if team != nil && team.IsK8s() {
			continue
		}
		paths = append(paths, filepath.Join(ta.TeamPath, ta.AgentName))
	}
	if len(paths) == 0 {
		return
	}

	claudeJSONPath := filepath.Join(home, ".claude.json")
	added, err := upsertClaudeJSONTrust(claudeJSONPath, paths)
	if err != nil {
		log.Printf("[daemon] warning: could not update local agent trust: %v", err)
		return
	}
	if added > 0 {
		log.Printf("[daemon] added trust entries for %d local agent workspaces", added)
	}
}

// bridgeEvents reads events from an adapter and routes them to Telegram.
func bridgeEvents(
	agentName, teamName string, adapter runtime.Adapter,
	mcfg *config.DaemonConfig, qs *questionStore, mt *messageTracker, msgSvc *message.Service,
) {
	ta, ok := mcfg.FindAgentInTeam(teamName, agentName)
	botToken := config.AgentBotToken(agentName)
	if !ok || botToken == "" {
		return
	}
	chatID := ta.ChatID

	go func() {
		for event := range adapter.Events() {
			switch event.Type {
			case runtime.EventText:
				rt := mcfg.AgentRuntimeForTeam(teamName, agentName)
				persistMsg(msgSvc, message.CreateParams{
					Sender: agentName, Recipient: mcfg.Global.UserName(), Content: event.Text,
					Team: teamName, Channel: message.ChannelAdapter, Runtime: &rt,
				})
				if err := telegram.SendMessage(botToken, chatID, event.Text); err != nil {
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
				// Check if emoji reactions are enabled for this team
				if team, ok := mcfg.Teams[teamName]; !ok || !team.EmojiReactions {
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
func handleSend(mcfg *config.DaemonConfig, registry *adapterRegistry, msgSvc *message.Service, req SendRequest) error {
	switch {
	case req.From != "" && req.To == "human":
		return handleFrom(mcfg, msgSvc, req)
	case req.From != "" && req.To != "":
		return handleAgentToAgent(mcfg, registry, msgSvc, req)
	case req.From != "":
		return handleFrom(mcfg, msgSvc, req)
	case req.To != "":
		return handleTo(mcfg, registry, msgSvc, req)
	default:
		return fmt.Errorf("send request missing from/to")
	}
}

// handleFrom sends a message from an agent to the human via Telegram.
func handleFrom(mcfg *config.DaemonConfig, msgSvc *message.Service, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.From)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	botToken := config.AgentBotToken(ta.AgentName)
	if botToken == "" {
		return fmt.Errorf("agent %s has no telegram configured", req.From)
	}
	rt := mcfg.AgentRuntimeForTeam(ta.TeamName, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: mcfg.Global.UserName(), Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
	return telegram.SendMessage(botToken, ta.ChatID, req.Message)
}

// handleTo delivers a message to an agent via its runtime adapter.
// Falls back to worker session delivery when the recipient is a hex UUID.
// Human→worker messages are sent as bare text (no [agent from:] prefix).
func handleTo(mcfg *config.DaemonConfig, registry *adapterRegistry, msgSvc *message.Service, req SendRequest) error {
	ta := resolveAgent(mcfg, req.Team, req.To)
	if ta == nil {
		session, err := resolveWorker(req.To)
		if err != nil {
			return fmt.Errorf("unknown agent or worker %s: %w", req.To, err)
		}
		log.Printf("[daemon] human-to-worker: %s → %s (%s)", mcfg.Global.UserName(), req.To, session)
		return dispatchToWorker(msgSvc, session, message.CreateParams{
			Sender: mcfg.Global.UserName(), Recipient: "worker:" + req.To,
			Content: req.Message, Team: req.Team, Channel: message.ChannelCLI,
		}, req.Message)
	}
	persistMsg(msgSvc, message.CreateParams{
		Sender: mcfg.Global.UserName(), Recipient: req.To, Content: req.Message,
		Team: ta.TeamName, Channel: message.ChannelCLI,
	})
	return deliverToAgent(registry, mcfg, ta.TeamName, req.To, req.Message)
}

// handleAgentToAgent delivers a message from one agent to another.
// Falls back to worker session delivery when the recipient is a hex UUID.
func handleAgentToAgent(
	mcfg *config.DaemonConfig, registry *adapterRegistry, msgSvc *message.Service, req SendRequest,
) error {
	fromTA := resolveAgent(mcfg, req.Team, req.From)
	if fromTA == nil {
		return fmt.Errorf("unknown agent: %s", req.From)
	}
	toTA := resolveAgent(mcfg, req.Team, req.To)
	msg := formatAgentMessage(req.From, req.Message)
	if toTA == nil {
		session, err := resolveWorker(req.To)
		if err != nil {
			return fmt.Errorf("unknown agent or worker %s: %w", req.To, err)
		}
		rt := mcfg.AgentRuntimeForTeam(fromTA.TeamName, req.From)
		log.Printf("[daemon] agent-to-worker: %s → %s (%s)", req.From, req.To, session)
		return dispatchToWorker(msgSvc, session, message.CreateParams{
			Sender: req.From, Recipient: "worker:" + req.To, Content: req.Message,
			Team: fromTA.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
		}, msg)
	}
	rt := mcfg.AgentRuntimeForTeam(fromTA.TeamName, req.From)
	persistMsg(msgSvc, message.CreateParams{
		Sender: req.From, Recipient: req.To, Content: req.Message,
		Team: fromTA.TeamName, Channel: message.ChannelCLI, Runtime: &rt,
	})
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

// workerWindow is the tmux window name used by all worker sessions.
const workerWindow = "worker"

// resolveWorker finds a tmux session for a worker identified by hex UUID prefix.
// Session names follow the format: w-{uuid[:8]}-{slug}.
// idPrefix must be at least 8 hex characters (case-insensitive).
func resolveWorker(idPrefix string) (string, error) {
	normalized := strings.ToLower(idPrefix)
	if len(normalized) < 8 {
		return "", fmt.Errorf("not a worker UUID: %q", idPrefix)
	}
	for _, c := range normalized {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return "", fmt.Errorf("not a worker UUID: %q", idPrefix)
		}
	}
	sessions, err := tmux.ListSessions()
	if err != nil {
		return "", fmt.Errorf("list tmux sessions: %w", err)
	}
	if sessions == nil {
		return "", fmt.Errorf("no tmux server running")
	}
	prefix := "w-" + normalized[:8]
	for _, s := range sessions {
		if strings.HasPrefix(s, prefix) {
			return s, nil
		}
	}
	return "", fmt.Errorf("no worker session for %s", idPrefix)
}

// dispatchToWorker persists a message and delivers it to a worker tmux session.
func dispatchToWorker(msgSvc *message.Service, session string, params message.CreateParams, text string) error {
	persistMsg(msgSvc, params)
	return deliverToWorker(session, text)
}

// deliverToWorker sends a message to a worker's tmux session.
func deliverToWorker(session, text string) error {
	return tmux.SendKeys(session, workerWindow, text)
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
func startWatcher(
	mcfg *config.DaemonConfig, qs *questionStore, mt *messageTracker, msgSvc *message.Service, done <-chan struct{},
) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[daemon] watcher disabled: cannot get home directory: %v — CC→Telegram bridging will not work", err)
		return
	}
	defaultProjectsDir := filepath.Join(home, ".claude", "projects")

	agentMap := make(map[string]watcher.WatchedAgent)
	for _, ta := range mcfg.AllAgents() {
		team := mcfg.Teams[ta.TeamName]

		var encoded, projectsDir string
		if team != nil && team.IsK8s() {
			// Container runs from /workspace/<agent> — encode that path
			encoded = watcher.EncodePath(filepath.Join("/workspace", ta.AgentName))
			// JSONL lives in the team's isolated .claude
			projectsDir = filepath.Join(home, ".ttal", ta.TeamName, ".claude", "projects")
		} else {
			encoded = watcher.EncodePath(filepath.Join(ta.TeamPath, ta.AgentName))
			projectsDir = defaultProjectsDir
		}

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
		func(teamName, agentName, correlationID string, questions []runtime.Question) {
			handleIncomingQuestion(qs, teamName, agentName, runtime.ClaudeCode, correlationID, questions, mcfg)
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
	botToken := config.AgentBotToken(agentName)
	if !ok || botToken == "" {
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
		BotToken:      botToken,
		CreatedAt:     time.Now(),
	}

	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)

	msgID, err := telegram.SendQuestionMessage(botToken, chatID, text, markup)
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

// shutdownAgents gracefully shuts down all agent sessions on daemon exit.
// K8s agent sessions receive /exit but pods are kept running for reuse.
// Local CC sessions are killed directly; status files are preserved so the
// next spawn can resume with --resume <session-id>.
func shutdownAgents(mcfg *config.DaemonConfig, registry *adapterRegistry) {
	registry.stopAll(context.Background())
	stopK8sAgents(mcfg)
	sessions := collectCCSessions(mcfg)
	if len(sessions) > 0 {
		shutdownCCSessions(sessions)
	}
}

// stopK8sAgents sends /exit to all k8s agent tmux sessions. Pods are kept running.
func stopK8sAgents(mcfg *config.DaemonConfig) {
	for teamName, pod := range k8sPods {
		for _, ta := range mcfg.AllAgents() {
			if ta.TeamName != teamName {
				continue
			}
			if err := pod.StopAgent(ta.AgentName); err != nil {
				log.Printf("[daemon] failed to stop k8s agent %s: %v", ta.AgentName, err)
			}
		}
	}
}

// collectCCSessions returns running CC tmux session names across all teams.
func collectCCSessions(mcfg *config.DaemonConfig) []string {
	var sessions []string
	for _, ta := range mcfg.AllAgents() {
		rt := mcfg.AgentRuntimeForTeam(ta.TeamName, ta.AgentName)
		if rt != runtime.ClaudeCode {
			continue
		}
		sessionName := config.AgentSessionName(ta.TeamName, ta.AgentName)
		if !tmux.SessionExists(sessionName) {
			continue
		}
		sessions = append(sessions, sessionName)
	}
	return sessions
}

// shutdownCCSessions kills CC tmux sessions directly.
func shutdownCCSessions(sessions []string) {
	for _, s := range sessions {
		if err := tmux.KillSession(s); err != nil {
			log.Printf("[daemon] failed to kill session %s: %v", s, err)
		} else {
			log.Printf("[daemon] killed CC session %s", s)
		}
	}
}

// agentProjectDir returns the ~/.claude/projects/<encoded> path for an agent.
func agentProjectDir(agentPath string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	encoded := watcher.EncodePath(agentPath)
	return filepath.Join(home, ".claude", "projects", encoded), nil
}

// ensureProjectDir creates the CC JSONL project directory for an agent.
// Called before spawnCCSession so the dir exists when CC starts and is
// ready for the watcher to monitor.
func ensureProjectDir(agentPath string) {
	dir, err := agentProjectDir(agentPath)
	if err != nil {
		log.Printf("[daemon] failed to resolve project dir for %s: %v", filepath.Base(agentPath), err)
		return
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		log.Printf("[daemon] failed to create project dir for %s: %v", filepath.Base(agentPath), err)
	}
}

// spawnCCSession creates a tmux session for a Claude Code agent.
func spawnCCSession(sessionName, agentName, agentPath, model, teamName string, env []string, shell string) error {
	cmd := "claude --dangerously-skip-permissions --agent " + agentName
	if model != "" {
		cmd += " --model " + model
	}
	if sid := lastSessionID(teamName, agentName, agentPath); sid != "" {
		cmd += " --resume " + sid
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

// sessionJSONLExists checks if a session's JSONL file exists in the
// project dir for the given agent path.
// Returns true on unexpected stat errors (conservative fallback — better to
// attempt --resume than silently drop it on a transient I/O error).
func sessionJSONLExists(sessionID, agentPath string) bool {
	dir, err := agentProjectDir(agentPath)
	if err != nil {
		return true // best-effort: assume exists
	}
	jsonlPath := filepath.Join(dir, sessionID+".jsonl")
	_, err = os.Stat(jsonlPath)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	log.Printf("[daemon] WARN: could not stat session JSONL %s: %v — assuming exists", jsonlPath, err)
	return true
}

// lastSessionID reads the persisted CC session ID for an agent from the status file.
// Returns "" on cold-start (no prior session), on read error (logged as WARN),
// or when the session's JSONL doesn't exist in the current project dir (CWD change).
func lastSessionID(teamName, agentName, agentPath string) string {
	s, err := status.ReadAgent(teamName, agentName)
	if err != nil {
		log.Printf("[daemon] WARN: could not read status for %s/%s, skipping --resume: %v", teamName, agentName, err)
		return ""
	}
	if s == nil {
		// Cold start — no prior session, nothing to resume.
		return ""
	}
	// Verify session JSONL exists in the current project dir.
	// After a CWD change the old session lives in a different encoded dir.
	if !sessionJSONLExists(s.SessionID, agentPath) {
		dir, _ := agentProjectDir(agentPath)
		log.Printf("[daemon] session %s not found in %s — starting fresh", s.SessionID, filepath.Base(dir))
		return ""
	}
	return s.SessionID
}

func writePID(path string) error {
	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
}

package frontend

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/humanfs"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/voice"
)

// chatTypeGroup and chatTypeSupergroup are Telegram chat type strings for group chats.
const (
	chatTypeGroup      = "group"
	chatTypeSupergroup = "supergroup"
)

// maxReplyContextLen is the maximum length of the reply context text before truncation.
const (
	maxReplyContextLen      = 200
	replyContextEllipsisLen = 3
)

// bashModePrefix is the prefix that triggers CC bash mode delivery.
// Messages starting with this are sent directly without the [telegram/matrix from:] wrapper.
const bashModePrefix = "! "

// pollerTarget groups agent info for Telegram poller dispatch by chat ID.
type pollerTarget struct {
	teamName  string
	agentName string
	chatID    string
}

// trackedMessage holds the Telegram message ID for the most recent
// inbound message to an agent, so reactions can be set on it.
type trackedMessage struct {
	ChatID    int64
	MessageID int
	BotToken  string
}

// messageTracker stores the most recent inbound Telegram message per agent.
// Key: "team:agent"
type messageTracker struct {
	mu    sync.RWMutex
	store map[string]trackedMessage
}

func newMessageTracker() *messageTracker {
	return &messageTracker{store: make(map[string]trackedMessage)}
}

func trackerKey(teamName, agentName string) string {
	return teamName + ":" + agentName
}

func (mt *messageTracker) set(teamName, agentName string, msg trackedMessage) {
	mt.mu.Lock()
	mt.store[trackerKey(teamName, agentName)] = msg
	mt.mu.Unlock()
}

func (mt *messageTracker) get(teamName, agentName string) (trackedMessage, bool) {
	mt.mu.RLock()
	msg, ok := mt.store[trackerKey(teamName, agentName)]
	mt.mu.RUnlock()
	return msg, ok
}

func (mt *messageTracker) delete(teamName, agentName string) {
	mt.mu.Lock()
	delete(mt.store, trackerKey(teamName, agentName))
	mt.mu.Unlock()
}

// TelegramFrontend implements Frontend using the Telegram Bot API.
type TelegramFrontend struct {
	cfg         TelegramConfig
	done        chan struct{}
	stopOnce    sync.Once
	ahs         *askHumanStore
	mt          *messageTracker
	allCommands []Command // set by RegisterCommands, used by Start
}

// NewTelegram creates a TelegramFrontend. RegisterCommands must be called before Start.
func NewTelegram(cfg TelegramConfig) *TelegramFrontend {
	return &TelegramFrontend{
		cfg: cfg,
		ahs: newAskHumanStore(),
		mt:  newMessageTracker(),
	}
}

// RegisterCommands stores the command list and calls Telegram setMyCommands for all
// agent bot tokens and the notification bot token in this team.
// Returns an aggregated error if any bot fails — callers may treat this as non-fatal.
func (f *TelegramFrontend) RegisterCommands(commands []Command) error {
	f.allCommands = commands

	allAgents := f.cfg.MCfg.Agents()
	tokenAgent := make(map[string]string)
	for _, ta := range allAgents {
		if false {
			continue
		}
		token := config.AgentBotToken(ta.AgentName)
		if token == "" {
			continue
		}
		if _, ok := tokenAgent[token]; !ok {
			tokenAgent[token] = ta.AgentName
		}
	}
	// Include notification bot token.
	if f.cfg.MCfg.NotificationToken != "" {
		if _, ok := tokenAgent[f.cfg.MCfg.NotificationToken]; !ok {
			tokenAgent[f.cfg.MCfg.NotificationToken] = "default-notify"
		}
	}

	var (
		wg    sync.WaitGroup
		errMu sync.Mutex
		errs  []error
	)
	for token, agentName := range tokenAgent {
		wg.Add(1)
		go func(tok, name string) {
			defer wg.Done()
			if err := f.registerBotCommands(tok); err != nil {
				log.Printf("[telegram] warning: failed to register bot commands for %s: %v", name, err)
				errMu.Lock()
				errs = append(errs, err)
				errMu.Unlock()
			} else {
				log.Printf("[telegram] registered bot commands for %s", name)
			}
		}(token, agentName)
	}
	wg.Wait()
	return errors.Join(errs...)
}

// Start begins Telegram polling for agents in this team.
// RegisterCommands must be called before Start.
func (f *TelegramFrontend) Start(ctx context.Context) error {
	f.done = make(chan struct{})
	go func() {
		<-ctx.Done()
		f.stopOnce.Do(func() { close(f.done) })
	}()
	f.startPollers()
	if err := f.StartNotificationPoller(ctx); err != nil {
		log.Printf("[telegram] StartNotificationPoller for team %s failed: %v", "default", err)
	}
	return nil
}

// ClearTracking clears the tracked inbound message for an agent.
// Called after the agent responds to prevent stale reactions on old messages.
func (f *TelegramFrontend) ClearTracking(_ context.Context, agentName string) error {
	f.mt.delete("default", agentName)
	return nil
}

// Stop shuts down the frontend. Calling Stop after ctx cancellation is a no-op.
func (f *TelegramFrontend) Stop(_ context.Context) error {
	if f.done != nil {
		f.stopOnce.Do(func() { close(f.done) })
	}
	return nil
}

// SendText sends a text message to an agent's Telegram chat.
func (f *TelegramFrontend) SendText(_ context.Context, agentName string, text string) error {
	ta := f.findAgent(agentName)
	if ta == nil {
		return fmt.Errorf("unknown agent: %s", agentName)
	}
	botToken := config.AgentBotToken(agentName)
	if botToken == "" {
		return fmt.Errorf("agent %s has no telegram bot token configured", agentName)
	}
	return telegram.SendMessage(botToken, f.cfg.MCfg.ChatID, text)
}

// SendVoice is not yet implemented for Telegram outbound (daemon → human).
func (f *TelegramFrontend) SendVoice(_ context.Context, _ string, _ []byte) error {
	return fmt.Errorf("SendVoice not implemented for TelegramFrontend")
}

// SendNotification sends a system notification to this team's notification channel.
func (f *TelegramFrontend) SendNotification(_ context.Context, text string) error {
	return notify.SendWithConfig(f.cfg.MCfg.NotificationToken, f.cfg.MCfg.ChatID, text)
}

// SetReaction sets an emoji reaction on the last tracked inbound message for an agent.
func (f *TelegramFrontend) SetReaction(_ context.Context, agentName string, emoji string) error {
	tracked, ok := f.mt.get("default", agentName)
	if !ok {
		return nil // no tracked message — silently skip
	}
	return telegram.SetReaction(tracked.BotToken, tracked.ChatID, tracked.MessageID, emoji)
}

// findAgent looks up a TeamAgent for the given agent name within this frontend's team.
func (f *TelegramFrontend) findAgent(agentName string) *config.AgentInfo {
	ta, ok := f.cfg.MCfg.FindAgent(agentName)
	if ok {
		return ta
	}
	return nil
}

// startPollers deduplicates agents by bot token and starts one poller per token.
func (f *TelegramFrontend) startPollers() {
	allAgents := f.cfg.MCfg.Agents()
	tokenTargets := buildTokenTargets(allAgents, f.cfg.MCfg.ChatID)
	for botToken, targets := range tokenTargets {
		dispatchMap := buildDispatchMap(targets)
		log.Printf("[telegram] starting multi-agent poller for %d agents on token ...%s",
			len(targets), botToken[len(botToken)-min(4, len(botToken)):])
		f.startPollerForToken(botToken, dispatchMap)
	}
}

func buildTokenTargets(allAgents []config.AgentInfo, teamChatID string) map[string][]pollerTarget {
	tokenTargets := make(map[string][]pollerTarget)
	for _, ta := range allAgents {
		token := config.AgentBotToken(ta.AgentName)
		if token == "" {
			log.Printf("[daemon] skipping telegram poller for %s: no bot_token", ta.AgentName)
			continue
		}
		tokenTargets[token] = append(tokenTargets[token], pollerTarget{
			teamName:  "default",
			agentName: ta.AgentName,
			chatID:    teamChatID,
		})
	}
	return tokenTargets
}

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

// startPollerForToken starts a long-poll loop for one bot token.
func (f *TelegramFrontend) startPollerForToken(botToken string, dispatch map[int64]pollerTarget) {
	go func() {
		backoff := 2 * time.Second
		for {
			select {
			case <-f.done:
				return
			default:
			}
			if err := f.runPoller(botToken, dispatch); err != nil {
				log.Printf("[telegram] poller failed: %v — retrying in %s", err, backoff)
				select {
				case <-f.done:
					return
				case <-time.After(backoff):
				}
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			} else {
				backoff = 2 * time.Second
			}
		}
	}()
}

func (f *TelegramFrontend) runPoller(botToken string, dispatch map[int64]pollerTarget) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-f.done
		cancel()
	}()

	// botUsername is populated after getMe; the handler closure captures it by reference.
	var botUsername string

	defaultHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		f.handleDefaultUpdate(ctx, b, update, dispatch, botToken, botUsername)
	}

	b, err := bot.New(botToken, bot.WithDefaultHandler(defaultHandler))
	if err != nil {
		return fmt.Errorf("bot init: %w", err)
	}

	me, err := b.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("getMe for token ...%s: %w", botToken[len(botToken)-min(4, len(botToken)):], err)
	}
	botUsername = strings.ToLower(me.Username)
	log.Printf("[telegram] bot identity: @%s", me.Username)

	// Register /restart for all agents sharing this token.
	b.RegisterHandlerMatchFunc(
		func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			if _, ok := dispatch[update.Message.Chat.ID]; !ok {
				return false
			}
			for _, e := range update.Message.Entities {
				if e.Type != models.MessageEntityTypeBotCommand || e.Offset != 0 {
					continue
				}
				raw := update.Message.Text[1:e.Length]
				name, _, _ := strings.Cut(raw, "@")
				if name == "restart" {
					return true
				}
			}
			return false
		},
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			if err := telegram.SendMessage(botToken, chatIDStr, "🔄 Daemon restarting..."); err != nil {
				log.Printf("[telegram] failed to send restart ack: %v", err)
			}
			if f.cfg.RestartFn != nil {
				if err := f.cfg.RestartFn(); err != nil {
					log.Printf("[telegram] restart failed: %v", err)
				}
			}
		},
	)

	for chatID, target := range dispatch {
		f.registerBotCommandsForAgent(b, target.agentName,
			botToken, target.chatID, chatID, botUsername, dispatch)
	}

	b.Start(ctx)
	return nil
}

func (f *TelegramFrontend) handleDefaultUpdate(
	ctx context.Context, b *bot.Bot, update *models.Update,
	dispatch map[int64]pollerTarget, botToken, botUsername string,
) {
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Type == models.MaybeInaccessibleMessageTypeMessage &&
			update.CallbackQuery.Message.Message != nil {
			chatID := update.CallbackQuery.Message.Message.Chat.ID
			if _, ok := dispatch[chatID]; ok {
				handleCallbackQuery(ctx, b, update.CallbackQuery, chatID, f.ahs)
			}
		}
		return
	}

	if update.Message == nil {
		return
	}
	if update.Message.From == nil {
		return
	}

	msg := update.Message
	switch msg.Chat.Type {
	case chatTypeGroup, chatTypeSupergroup:
		target := resolveGroupTarget(msg, botUsername, dispatch)
		if target == nil {
			return
		}
		if msg.Text != "" {
			if interceptedAsHumanAnswer(msg, f.ahs) {
				return
			}
		}
		stripped := stripFirstBotMention(msg, botUsername)
		f.handleInboundMessage(
			ctx, b, msg,
			target.teamName, target.agentName, botToken, target.chatID,
			func(agentName, text string) {
				f.cfg.OnMessage(target.teamName, agentName, text)
			},
			&stripped,
		)
	default:
		target, ok := dispatch[msg.Chat.ID]
		if !ok {
			return
		}
		if msg.Text != "" {
			if interceptedAsHumanAnswer(msg, f.ahs) {
				return
			}
		}
		f.handleInboundMessage(
			ctx, b, msg,
			target.teamName, target.agentName, botToken, target.chatID,
			func(agentName, text string) {
				f.cfg.OnMessage(target.teamName, agentName, text)
			},
			nil,
		)
	}
}

func (f *TelegramFrontend) handleInboundMessage(
	ctx context.Context, b *bot.Bot, msg *models.Message,
	teamName, agentName, botToken, chatIDStr string,
	onMessage func(string, string),
	overrideText *string,
) {
	// Track this message for tool reactions.
	if chatID, err := telegram.ParseChatID(chatIDStr); err != nil {
		log.Printf("[telegram] BUG: failed to parse chatIDStr %q for agent %s: %v", chatIDStr, agentName, err)
	} else {
		f.mt.set(teamName, agentName, trackedMessage{
			ChatID:    chatID,
			MessageID: msg.ID,
			BotToken:  botToken,
		})
	}

	senderName := f.cfg.UserNameFn()
	if senderName == "" {
		senderName = msg.From.Username
		if senderName == "" {
			senderName = msg.From.FirstName
		}
	}

	replyCtx := extractReplyContext(msg)

	// Handle voice messages.
	if msg.Voice != nil {
		transcription, err := transcribeVoiceMessage(ctx, b, msg.Voice)
		if err != nil {
			log.Printf("[telegram] voice transcription failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "Voice transcription failed — check daemon logs for details")
			return
		}
		rawText := "[🎤 voice] " + transcription
		f.persistInbound(senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(senderName, replyCtx+rawText))
		return
	}

	// Handle photo messages.
	if len(msg.Photo) > 0 {
		photo := msg.Photo[len(msg.Photo)-1]
		filename := fmt.Sprintf("photo_%d.jpg", msg.ID)
		localPath, err := downloadTelegramFile(ctx, b, photo.FileID, teamName, agentName, filename)
		if err != nil {
			log.Printf("[telegram] photo download failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "Photo download failed — check daemon logs for details")
			return
		}
		rawText := fmt.Sprintf("[📷 photo] %s", localPath)
		if caption := msg.Caption; caption != "" {
			rawText += " " + caption
		}
		f.persistInbound(senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(senderName, replyCtx+rawText))
		return
	}

	// Handle document/file messages.
	if msg.Document != nil {
		filename := msg.Document.FileName
		if filename == "" {
			filename = fmt.Sprintf("file_%d", msg.ID)
		}
		localPath, err := downloadTelegramFile(ctx, b, msg.Document.FileID, teamName, agentName, filename)
		if err != nil {
			log.Printf("[telegram] document download failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "File download failed — check daemon logs for details")
			return
		}
		rawText := fmt.Sprintf("[📎 file] %s", localPath)
		if caption := msg.Caption; caption != "" {
			rawText += " " + caption
		}
		f.persistInbound(senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(senderName, replyCtx+rawText))
		return
	}

	var text string
	if overrideText != nil {
		text = *overrideText
	} else {
		text = strings.TrimSpace(msg.Text)
	}
	f.persistInbound(senderName, agentName, teamName, text)

	// Bash mode: "! " prefix sends directly to CC without [telegram from:] wrapper.
	if strings.HasPrefix(text, bashModePrefix) {
		onMessage(agentName, text)
		return
	}

	onMessage(agentName, formatInboundMessage(senderName, replyCtx+text))
}

func (f *TelegramFrontend) persistInbound(sender, recipient, team, content string) {
	if f.cfg.MsgSvc == nil {
		return
	}
	if _, err := f.cfg.MsgSvc.Create(context.Background(), message.CreateParams{
		Sender: sender, Recipient: recipient, Content: content,
		Team: team, Channel: message.ChannelTelegram,
	}); err != nil {
		log.Printf("[telegram] message persist failed (sender=%s): %v", sender, err)
	}
}

// formatInboundMessage formats a Telegram message for delivery to the agent.
func formatInboundMessage(senderName, text string) string {
	return fmt.Sprintf(
		"[telegram from:%s] %s\n\n<i>--- Reply with: ttal send --to human \"your message\"</i>",
		senderName, text)
}

// registerBotCommands calls Telegram setMyCommands API for this bot token.
func (f *TelegramFrontend) registerBotCommands(botToken string) error {
	return registerBotCommands(botToken, f.allCommands)
}

// registerBotCommandsForAgent registers each bot command handler on the bot instance for an agent.
func (f *TelegramFrontend) registerBotCommandsForAgent(
	b *bot.Bot, agentName, botToken, chatIDStr string,
	chatID int64, botUsername string, dispatch map[int64]pollerTarget,
) {
	matchCommand := func(cmd string) bot.MatchFunc {
		return func(update *models.Update) bool {
			if update.Message == nil {
				return false
			}
			return isBotCommandForAgent(update.Message, cmd, chatID, botUsername, dispatch)
		}
	}

	b.RegisterHandlerMatchFunc(matchCommand("status"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			args := parseCommandArgs(update.Message.Text)
			f.handleStatusCommand(agentName, botToken, chatIDStr, args)
		})

	b.RegisterHandlerMatchFunc(matchCommand("help"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			f.handleHelpCommand(botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("usage"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			f.handleUsageCommand(botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("new"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			fullCmd := buildSlashCommand("new", update.Message.Text)
			sendKeysToAgent(agentName, botToken, chatIDStr, fullCmd, "Sent /new — starting fresh conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("compact"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			fullCmd := buildSlashCommand("compact", update.Message.Text)
			sendKeysToAgent(agentName, botToken, chatIDStr, fullCmd, "Sent /compact — compacting conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("wait"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			sendEscToAgent(agentName, botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("save"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			args := parseCommandArgs(update.Message.Text)
			f.handleSaveCommand(agentName, botToken, chatIDStr, args)
		})

	for _, cmd := range f.allCommands {
		if isStaticCommand(cmd.Name) {
			continue
		}
		cmdName := cmd.Name
		origName := cmd.OriginalName
		if origName == "" {
			origName = cmdName
		}
		b.RegisterHandlerMatchFunc(matchCommand(cmdName),
			func(_ context.Context, _ *bot.Bot, update *models.Update) {
				skillCmd := buildSkillGetCommand(origName, update.Message.Text)
				sendKeysToAgent(agentName, botToken, chatIDStr, skillCmd, "")
			})
	}
}

// --- Bot command handlers ---

func (f *TelegramFrontend) handleStatusCommand(_ string, botToken, chatID string, args []string) {
	var agents []status.AgentStatus

	if len(args) > 0 {
		s, err := status.ReadAgent("default", args[0])
		if err != nil {
			replyTelegram(botToken, chatID, "Error: "+err.Error())
			return
		}
		if s == nil {
			replyTelegram(botToken, chatID, args[0]+": no status data")
			return
		}
		agents = []status.AgentStatus{*s}
	} else {
		all, err := status.ReadAll("default")
		if err != nil {
			replyTelegram(botToken, chatID, "Error reading status: "+err.Error())
			return
		}
		agents = all
	}

	if len(agents) == 0 {
		replyTelegram(botToken, chatID, "No agent status data available")
		return
	}

	teamPath := f.cfg.MCfg.TeamPath

	sort.Slice(agents, func(i, j int) bool { return agents[i].ContextUsedPct > agents[j].ContextUsedPct })
	var sb strings.Builder
	for _, a := range agents {
		staleMarker := ""
		if a.IsStale(5 * time.Minute) {
			staleMarker = " (stale)"
		}

		emoji := ""
		role := ""
		if teamPath != "" {
			if meta, err := agentfs.Get(teamPath, a.Agent); err == nil {
				if meta.Emoji != "" {
					emoji = meta.Emoji + " "
				}
				if meta.Role != "" {
					role = " (" + meta.Role + ")"
				}
			}
		}

		fmt.Fprintf(&sb, "%s%s%s — %.0f%% ctx%s\n", emoji, a.Agent, role, a.ContextUsedPct, staleMarker)
	}
	replyTelegram(botToken, chatID, sb.String())
}

func (f *TelegramFrontend) handleHelpCommand(botToken, chatID string) {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, cmd := range f.allCommands {
		fmt.Fprintf(&sb, "/%s — %s\n", cmd.Name, cmd.Description)
	}
	sb.WriteString("\nAnything else is sent as a message to the agent.")
	replyTelegram(botToken, chatID, sb.String())
}

func (f *TelegramFrontend) handleUsageCommand(botToken, chatID string) {
	if f.cfg.GetUsageFn == nil {
		replyTelegram(botToken, chatID, "Usage data not yet available — daemon is still fetching")
		return
	}
	msg := f.cfg.GetUsageFn()
	if msg == "" {
		replyTelegram(botToken, chatID, "Usage data not yet available — daemon is still fetching")
		return
	}
	replyTelegram(botToken, chatID, msg)
}

// flicknoteIDPattern extracts the note ID from flicknote add output.
var flicknoteIDPattern = regexp.MustCompile(`Created note ([0-9a-f]+)`)

func (f *TelegramFrontend) handleSaveCommand(agentName, botToken, chatID string, args []string) {
	if f.cfg.MsgSvc == nil {
		replyTelegram(botToken, chatID, "Error: message service not available")
		return
	}

	dbCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer dbCancel()
	msg, err := f.cfg.MsgSvc.LatestFrom(dbCtx, agentName, "default")
	if err != nil {
		replyTelegram(botToken, chatID, "Error reading last message: "+err.Error())
		return
	}
	if msg == nil {
		replyTelegram(botToken, chatID, "No messages from "+agentName+" to save")
		return
	}

	// Default project is "saved"; /save <project> overrides it
	project := "saved"
	if len(args) > 0 {
		project = args[0]
	}

	flickCtx, flickCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer flickCancel()
	cmd := exec.CommandContext(flickCtx, "flicknote", "add", "--project", project)
	cmd.Stdin = strings.NewReader(msg.Content)
	out, err := cmd.CombinedOutput()
	if err != nil {
		replyTelegram(botToken, chatID, fmt.Sprintf("flicknote add failed: %v\n%s", err, string(out)))
		return
	}

	// Extract note ID from output (e.g. "Created note c9b5979b in project ...")
	outStr := strings.TrimSpace(string(out))
	noteID := ""
	if m := flicknoteIDPattern.FindStringSubmatch(outStr); len(m) > 1 {
		noteID = m[1]
	}

	if noteID != "" {
		replyTelegram(botToken, chatID, fmt.Sprintf("💾 Saved to flicknote/%s (project: %s)", noteID, project))
	} else {
		replyTelegram(botToken, chatID, fmt.Sprintf("💾 Saved to flicknote (project: %s)\n%s", project, outStr))
	}
}

// --- Package-level helpers ---

func replyTelegram(botToken, chatID, text string) {
	if err := telegram.SendMessage(botToken, chatID, text); err != nil {
		log.Printf("[telegram] reply failed: %v", err)
	}
}

func sendKeysToAgent(agentName, botToken, chatID, keys, confirmMsg string) {
	session := config.AgentSessionName(agentName)
	if err := tmux.SendKeys(session, agentName, keys); err != nil {
		replyTelegram(botToken, chatID, "Error: "+err.Error())
		return
	}
	if confirmMsg != "" {
		replyTelegram(botToken, chatID, confirmMsg)
	}
}

func sendEscToAgent(agentName, botToken, chatID string) {
	session := config.AgentSessionName(agentName)
	if err := tmux.SendRawKey(session, agentName, "Escape"); err != nil {
		replyTelegram(botToken, chatID, "Error: "+err.Error())
		return
	}
	replyTelegram(botToken, chatID, "Sent Escape — interrupting agent")
}

func parseCommandArgs(text string) []string {
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

func joinArgs(args []string, separator string) string {
	if len(args) == 0 {
		return ""
	}
	return separator + strings.Join(args, " ")
}

func buildSlashCommand(cmdName, messageText string) string {
	args := parseCommandArgs(messageText)
	return "/" + cmdName + joinArgs(args, " ")
}

// buildSkillGetCommand builds a `run skill get <name>` command for dynamic skills.
// Any trailing arguments from the message are appended after the skill name,
// separated by a newline so the agent sees them as follow-up context.
func buildSkillGetCommand(skillName, messageText string) string {
	args := parseCommandArgs(messageText)
	cmd := "run skill get " + skillName
	if len(args) > 0 {
		cmd += "\n" + strings.Join(args, " ")
	}
	return cmd
}

func isStaticCommand(name string) bool {
	static := []string{"status", "usage", "new", "compact", "wait", "restart", "help", "save"}
	for _, s := range static {
		if s == name {
			return true
		}
	}
	return false
}

// registerBotCommands calls Telegram setMyCommands API.
func registerBotCommands(botToken string, commands []Command) error {
	type tgCmd struct {
		Command     string `json:"command"`
		Description string `json:"description"`
	}
	cmds := make([]tgCmd, 0, len(commands))
	for _, c := range commands {
		cmds = append(cmds, tgCmd{Command: c.Name, Description: c.Description})
	}

	body, err := json.Marshal(map[string]interface{}{"commands": cmds})
	if err != nil {
		return fmt.Errorf("marshal commands: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/setMyCommands", botToken)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("setMyCommands request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("setMyCommands returned %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// isBotCommandForAgent checks whether msg contains the given bot command addressed to this agent.
func isBotCommandForAgent(
	msg *models.Message, cmd string, chatID int64, botUsername string, dispatch map[int64]pollerTarget,
) bool {
	isGroup := msg.Chat.Type == chatTypeGroup || msg.Chat.Type == chatTypeSupergroup
	if !isGroup && msg.Chat.ID != chatID {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type != models.MessageEntityTypeBotCommand || e.Offset != 0 {
			continue
		}
		raw := msg.Text[1:e.Length]
		name, atBot, hasAt := strings.Cut(raw, "@")
		if name != cmd {
			continue
		}
		if isGroup {
			if !hasAt || !strings.EqualFold(atBot, botUsername) {
				return false
			}
			if _, ok := dispatch[msg.Chat.ID]; !ok {
				return false
			}
		}
		return true
	}
	return false
}

// resolveGroupTarget determines which agent should handle a group chat message.
func resolveGroupTarget(msg *models.Message, botUsername string, dispatch map[int64]pollerTarget) *pollerTarget {
	if botUsername == "" {
		return nil
	}

	addressed := msg.ReplyToMessage != nil &&
		msg.ReplyToMessage.From != nil &&
		msg.ReplyToMessage.From.IsBot &&
		strings.EqualFold(msg.ReplyToMessage.From.Username, botUsername)

	if !addressed {
		s, _ := findFirstBotMention(msg, botUsername)
		addressed = s >= 0
	}

	if !addressed {
		return nil
	}

	target, ok := dispatch[msg.Chat.ID]
	if !ok {
		log.Printf("[telegram] WARNING: group message for @%s from chat %d — no matching agent (%d registered)",
			botUsername, msg.Chat.ID, len(dispatch))
		return nil
	}
	return &target
}

// utf16OffsetToRuneIdx converts a UTF-16 code unit offset to a rune index.
func utf16OffsetToRuneIdx(s string, utf16Off int) int {
	u16 := 0
	ri := 0
	for _, r := range s {
		if u16 >= utf16Off {
			return ri
		}
		if r >= 0x10000 {
			u16 += 2
		} else {
			u16++
		}
		ri++
	}
	return ri
}

// findFirstBotMention returns the rune-index range [start, end) of the first @botUsername mention.
func findFirstBotMention(msg *models.Message, botUsername string) (start, end int) {
	if msg.Text == "" || botUsername == "" {
		return -1, -1
	}
	runes := []rune(msg.Text)
	for _, entity := range msg.Entities {
		if entity.Type != models.MessageEntityTypeMention {
			continue
		}
		s := utf16OffsetToRuneIdx(msg.Text, entity.Offset)
		e := utf16OffsetToRuneIdx(msg.Text, entity.Offset+entity.Length)
		if s < 0 || e > len(runes) || s+1 >= e {
			continue
		}
		mentioned := strings.ToLower(string(runes[s+1 : e]))
		if mentioned == botUsername {
			return s, e
		}
	}
	return -1, -1
}

// stripFirstBotMention removes the first @botUsername mention from msg.Text.
func stripFirstBotMention(msg *models.Message, botUsername string) string {
	if botUsername == "" {
		return msg.Text
	}
	s, e := findFirstBotMention(msg, botUsername)
	if s < 0 {
		return msg.Text
	}
	runes := []rune(msg.Text)
	return strings.TrimSpace(string(runes[:s]) + string(runes[e:]))
}

// extractReplyContext extracts the text from a replied-to message as a formatted prefix.
func extractReplyContext(msg *models.Message) string {
	if msg == nil || msg.ReplyToMessage == nil {
		return ""
	}

	reply := msg.ReplyToMessage
	var text string
	switch {
	case reply.Text != "":
		text = reply.Text
	case reply.Caption != "":
		text = reply.Caption
	case reply.Voice != nil:
		text = "[voice message]"
	case reply.Audio != nil:
		text = "[audio: " + reply.Audio.FileName + "]"
	case reply.Document != nil:
		text = "[file: " + reply.Document.FileName + "]"
	case len(reply.Photo) > 0:
		text = "[photo]"
	case reply.Video != nil:
		text = "[video]"
	case reply.Sticker != nil:
		text = "[sticker: " + reply.Sticker.Emoji + "]"
	default:
		text = "[message]"
	}

	if len(text) > maxReplyContextLen {
		text = text[:maxReplyContextLen-replyContextEllipsisLen] + "..."
	}
	return fmt.Sprintf("[replying to: '%s'] ", text)
}

// transcribeVoiceMessage downloads and transcribes a Telegram voice message.
func transcribeVoiceMessage(ctx context.Context, b *bot.Bot, v *models.Voice) (string, error) {
	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: v.FileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	fileURL := b.FileDownloadLink(file)
	resp, err := http.Get(fileURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download voice: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download voice: HTTP %d", resp.StatusCode)
	}
	audioData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read voice data: %w", err)
	}
	return voice.Transcribe(audioData, "voice.ogg")
}

// downloadTelegramFile downloads a file from Telegram and saves it locally.
func downloadTelegramFile(
	ctx context.Context, b *bot.Bot,
	fileID, teamName, agentName, filename string,
) (string, error) {
	filename = filepath.Base(filename)
	teamName = filepath.Base(teamName)
	agentName = filepath.Base(agentName)

	if teamName == "" || teamName == "." {
		log.Printf("[telegram] warning: empty teamName for agent %s in file download", agentName)
	}

	file, err := b.GetFile(ctx, &bot.GetFileParams{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("get file: %w", err)
	}
	fileURL := b.FileDownloadLink(file)
	resp, err := http.Get(fileURL) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("download file: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download file: HTTP %d", resp.StatusCode)
	}

	dir := filepath.Join(config.DefaultDataDir(), "files", teamName, agentName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create file dir: %w", err)
	}
	localPath := filepath.Join(dir, filename)
	if _, err := os.Stat(localPath); err == nil {
		ext := filepath.Ext(filename)
		base := strings.TrimSuffix(filename, ext)
		localPath = filepath.Join(dir, fmt.Sprintf("%s_%d%s", base, time.Now().UnixMilli(), ext))
	}
	out, err := os.Create(localPath) //nolint:gosec
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := out.Close(); err != nil {
		return "", fmt.Errorf("close file: %w", err)
	}
	return localPath, nil
}

// SendToHuman sends a message to a human's Telegram chat using the notification bot token.
func (f *TelegramFrontend) SendToHuman(_ context.Context, human *humanfs.Human, text string) error {
	if human.TelegramChatID == "" {
		return fmt.Errorf("human %s has no telegram_chat_id configured", human.Alias)
	}
	// Use the notification bot token for human delivery
	botToken := f.cfg.MCfg.NotificationToken
	if botToken == "" {
		return fmt.Errorf("no notification bot token configured for human %s", human.Alias)
	}
	return telegram.SendMessage(botToken, human.TelegramChatID, text)
}

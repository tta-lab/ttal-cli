package daemon

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/voice"
)

// chatTypeGroup and chatTypeSupergroup are the Telegram chat type strings for group chats.
const (
	chatTypeGroup      = "group"
	chatTypeSupergroup = "supergroup"
)

// startMultiAgentPoller starts a long-poll loop for one bot token serving multiple agents.
// Dispatches messages by chat ID to the correct agent.
// Runs until done is closed.
func startMultiAgentPoller(
	botToken string,
	dispatch map[int64]pollerTarget,
	onMessage func(teamName, agentName, text string), done <-chan struct{},
	ahs *askHumanStore,
	allCommands []BotCommand, mt *messageTracker, msgSvc *message.Service,
	userNameFn func(teamName string) string,
) {
	go func() {
		backoff := 2 * time.Second

		for {
			select {
			case <-done:
				return
			default:
			}

			if err := runMultiAgentPoller(
				botToken, dispatch, onMessage, done, ahs, allCommands, mt, msgSvc, userNameFn,
			); err != nil {
				log.Printf("[telegram] poller failed: %v — retrying in %s", err, backoff)
				select {
				case <-done:
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

func runMultiAgentPoller(
	botToken string,
	dispatch map[int64]pollerTarget,
	onMessage func(teamName, agentName, text string), done <-chan struct{},
	ahs *askHumanStore,
	allCommands []BotCommand, mt *messageTracker, msgSvc *message.Service,
	userNameFn func(teamName string) string,
) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel context when done is closed
	go func() {
		<-done
		cancel()
	}()

	// botUsername is populated after getMe; the handler closure captures it by reference.
	// Messages only arrive after b.Start(ctx), so the value is set before any dispatch.
	var botUsername string

	defaultHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		handleDefaultUpdate(ctx, b, update, dispatch, botToken, botUsername,
			onMessage, ahs, mt, msgSvc, userNameFn)
	}

	b, err := bot.New(botToken, bot.WithDefaultHandler(defaultHandler))
	if err != nil {
		return fmt.Errorf("bot init: %w", err)
	}

	// Discover this bot's username for group chat @mention routing.
	// Return on failure — the caller's retry loop will restart the poller.
	me, err := b.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("getMe for token ...%s: %w", botToken[len(botToken)-min(4, len(botToken)):], err)
	}
	botUsername = strings.ToLower(me.Username)
	log.Printf("[telegram] bot identity: @%s", me.Username)

	// Register /restart once per bot token (global, not per-agent).
	// Any authorized chat (present in dispatch) can trigger it.
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
			if err := Restart(); err != nil {
				log.Printf("[telegram] restart failed: %v", err)
			}
		},
	)

	// Register bot commands for ALL agents sharing this token.
	// Each handler checks chat ID to route to the correct agent.
	for chatID, target := range dispatch {
		registerBotCommandsForAgent(b, target.teamName, target.agentName,
			botToken, target.chatID, chatID, botUsername, dispatch, allCommands)
	}

	b.Start(ctx)
	return nil
}

func handleDefaultUpdate(
	ctx context.Context, b *bot.Bot, update *models.Update,
	dispatch map[int64]pollerTarget, botToken, botUsername string,
	onMessage func(teamName, agentName, text string),
	ahs *askHumanStore,
	mt *messageTracker, msgSvc *message.Service,
	userNameFn func(teamName string) string,
) {
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Type == models.MaybeInaccessibleMessageTypeMessage &&
			update.CallbackQuery.Message.Message != nil {
			chatID := update.CallbackQuery.Message.Message.Chat.ID
			if _, ok := dispatch[chatID]; ok {
				handleCallbackQuery(ctx, b, update.CallbackQuery, chatID, ahs)
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
			if interceptedAsHumanAnswer(msg, ahs) {
				return
			}
		}
		// Strip the @mention from the delivered text so the agent sees a clean message.
		// Use *string so handleInboundMessage can distinguish an empty post-strip result
		// (e.g. message was purely "@botname") from "no override provided".
		stripped := stripFirstBotMention(msg, botUsername)
		handleInboundMessage(
			ctx, b, msg,
			target.teamName, target.agentName, botToken, target.chatID,
			func(agentName, text string) {
				onMessage(target.teamName, agentName, text)
			},
			mt, msgSvc, &stripped, userNameFn,
		)
	default:
		// DM / private: route by chat ID (existing behaviour, unchanged).
		target, ok := dispatch[msg.Chat.ID]
		if !ok {
			return
		}
		if msg.Text != "" {
			if interceptedAsHumanAnswer(msg, ahs) {
				return
			}
		}
		handleInboundMessage(
			ctx, b, msg,
			target.teamName, target.agentName, botToken, target.chatID,
			func(agentName, text string) {
				onMessage(target.teamName, agentName, text)
			},
			mt, msgSvc, nil, userNameFn,
		)
	}
}

// utf16OffsetToRuneIdx converts a UTF-16 code unit offset to a rune index in s.
// Telegram entity offsets are UTF-16 code units; non-BMP characters (e.g. emoji)
// occupy 2 code units but count as 1 rune, so naive byte/rune indexing is incorrect.
func utf16OffsetToRuneIdx(s string, utf16Off int) int {
	u16 := 0
	ri := 0
	for _, r := range s {
		if u16 >= utf16Off {
			return ri
		}
		if r >= 0x10000 {
			u16 += 2 // surrogate pair
		} else {
			u16++
		}
		ri++
	}
	return ri
}

// findFirstBotMention returns the rune-index range [start, end) of the first
// @botUsername mention entity in msg. Returns (-1, -1) if none found.
// Uses UTF-16–aware offset conversion so emoji before the mention are handled correctly.
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
		// Entity covers "@username"; skip the leading '@'.
		mentioned := strings.ToLower(string(runes[s+1 : e]))
		if mentioned == botUsername {
			return s, e
		}
	}
	return -1, -1
}

// resolveGroupTarget determines which agent should handle a group chat message.
// Returns nil if the message doesn't address any known bot in the dispatch map.
//
// Priority 1: Reply to a message from this bot.
// Priority 2: First @mention of this bot's username in the message entities.
func resolveGroupTarget(msg *models.Message, botUsername string, dispatch map[int64]pollerTarget) *pollerTarget {
	if botUsername == "" {
		return nil
	}

	// Priority 1: reply to this bot's own message.
	addressed := msg.ReplyToMessage != nil &&
		msg.ReplyToMessage.From != nil &&
		msg.ReplyToMessage.From.IsBot &&
		strings.EqualFold(msg.ReplyToMessage.From.Username, botUsername)

	// Priority 2: first @mention of this bot.
	if !addressed {
		s, _ := findFirstBotMention(msg, botUsername)
		addressed = s >= 0
	}

	if !addressed {
		return nil
	}

	// Route by chat ID — same as DM path.
	target, ok := dispatch[msg.Chat.ID]
	if !ok {
		log.Printf("[telegram] WARNING: group message for @%s from chat %d — no matching agent (%d registered)",
			botUsername, msg.Chat.ID, len(dispatch))
		return nil
	}
	return &target
}

// stripFirstBotMention removes the first @botUsername mention from msg.Text and
// trims surrounding whitespace. Returns msg.Text unchanged if no mention is found.
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

func handleInboundMessage(
	ctx context.Context, b *bot.Bot, msg *models.Message,
	teamName, agentName, botToken, chatIDStr string,
	onMessage func(string, string),
	mt *messageTracker, msgSvc *message.Service,
	overrideText *string,
	userNameFn func(teamName string) string,
) {
	// Track this message for tool reactions
	if mt != nil {
		chatID, err := telegram.ParseChatID(chatIDStr)
		if err != nil {
			log.Printf("[telegram] BUG: failed to parse chatIDStr %q for agent %s: %v", chatIDStr, agentName, err)
		} else {
			mt.set(teamName, agentName, trackedMessage{
				ChatID:    chatID,
				MessageID: msg.ID,
				BotToken:  botToken,
			})
		}
	}

	// Normalize to the configured human identity so the GUI can match both sides of a
	// conversation (inbound sender vs. outbound recipient both resolve to the same name).
	// Falls back to the actual Telegram username when no identity is configured.
	// Note: this assumes inbound messages to the bot are from the team's primary human user.
	// Multi-human group scenarios would require a Telegram-username→internal-name mapping.
	senderName := userNameFn(teamName)
	if senderName == "" {
		senderName = msg.From.Username
		if senderName == "" {
			senderName = msg.From.FirstName
		}
	}

	// Extract reply context if this message is a reply
	replyCtx := extractReplyContext(msg)

	// Handle voice messages
	if msg.Voice != nil {
		transcription, err := transcribeVoiceMessage(ctx, b, msg.Voice)
		if err != nil {
			log.Printf("[telegram] voice transcription failed for %s: %v", agentName, err)
			_ = telegram.SendMessage(botToken, chatIDStr, "Voice transcription failed — check daemon logs for details")
			return
		}
		rawText := "[🎤 voice] " + transcription
		persistInbound(msgSvc, senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+rawText))
		return
	}

	// Handle photo messages
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
		persistInbound(msgSvc, senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+rawText))
		return
	}

	// Handle document/file messages
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
		persistInbound(msgSvc, senderName, agentName, teamName, rawText)
		onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+rawText))
		return
	}

	var text string
	if overrideText != nil {
		text = *overrideText
	} else {
		text = strings.TrimSpace(msg.Text)
	}
	persistInbound(msgSvc, senderName, agentName, teamName, text)
	onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+text))
}

// isBotCommandForAgent reports whether msg contains the given bot command addressed to
// this agent. For DMs, it validates by chatID. For groups, it requires the @botname
// suffix and that the group chatID is a registered dispatch target.
func isBotCommandForAgent(
	msg *models.Message, cmd string, chatID int64, botUsername string, dispatch map[int64]pollerTarget,
) bool {
	isGroup := msg.Chat.Type == chatTypeGroup || msg.Chat.Type == chatTypeSupergroup
	// In private/other: require matching DM chat ID.
	if !isGroup && msg.Chat.ID != chatID {
		return false
	}
	for _, e := range msg.Entities {
		if e.Type != models.MessageEntityTypeBotCommand || e.Offset != 0 {
			continue
		}
		// Extract command name: skip leading "/", strip @botname suffix.
		raw := msg.Text[1:e.Length]
		name, atBot, hasAt := strings.Cut(raw, "@")
		if name != cmd {
			continue
		}
		// In groups: @botname suffix MUST be present and match, AND the
		// group chatID must be a registered dispatch target. This ensures
		// commands only execute from explicitly authorized group chats.
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

// registerBotCommandsForAgent registers each bot command as a handler on the bot instance.
// Uses RegisterHandlerMatchFunc with a custom matcher that:
//   - Checks for bot_command entity at message start (like MatchTypeCommandStartOnly)
//   - Strips @botname suffix from the command for group chat compatibility
//   - Validates chat ID so commands only work from the configured chat
func registerBotCommandsForAgent(
	b *bot.Bot, teamName, agentName, botToken, chatIDStr string,
	chatID int64, botUsername string, dispatch map[int64]pollerTarget, allCommands []BotCommand,
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
			handleStatusCommand(teamName, agentName, botToken, chatIDStr, args)
		})

	b.RegisterHandlerMatchFunc(matchCommand("help"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			handleHelpCommand(botToken, chatIDStr, allCommands)
		})

	b.RegisterHandlerMatchFunc(matchCommand("usage"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			handleUsageCommand(botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("new"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			fullCmd := buildFullCommand("new", update.Message.Text)
			sendKeysToAgent(teamName, agentName, botToken, chatIDStr, fullCmd, "Sent /new — starting fresh conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("compact"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			fullCmd := buildFullCommand("compact", update.Message.Text)
			sendKeysToAgent(teamName, agentName, botToken, chatIDStr, fullCmd, "Sent /compact — compacting conversation")
		})

	b.RegisterHandlerMatchFunc(matchCommand("wait"),
		func(_ context.Context, _ *bot.Bot, _ *models.Update) {
			sendEscToAgent(teamName, agentName, botToken, chatIDStr)
		})

	// Register discovered commands — forward as "Use command skill. args" to agent's tmux pane.
	// Use OriginalName (with hyphens) for dispatch since Claude Code skills
	// use hyphenated names, but match on Command (sanitized with underscores).
	for _, cmd := range allCommands {
		if isStaticCommand(cmd.Command) {
			continue
		}
		cmdName := cmd.Command       // sanitized name for Telegram matching
		origName := cmd.OriginalName // original name for agent dispatch
		if origName == "" {
			origName = cmdName
		}
		b.RegisterHandlerMatchFunc(matchCommand(cmdName),
			func(_ context.Context, _ *bot.Bot, update *models.Update) {
				fullCmd := buildSkillCommand(origName, update.Message.Text)
				sendKeysToAgent(teamName, agentName, botToken, chatIDStr, fullCmd, "")
			})
	}
}

// parseCommandArgs extracts arguments after a /command from message text.
// e.g. "/status yuki" → ["yuki"]
func parseCommandArgs(text string) []string {
	parts := strings.Fields(text)
	if len(parts) <= 1 {
		return nil
	}
	return parts[1:]
}

// joinArgs joins parsed arguments with the given separator.
func joinArgs(args []string, separator string) string {
	if len(args) == 0 {
		return ""
	}
	return separator + strings.Join(args, " ")
}

// buildFullCommand constructs a slash command string from a command name and
// the raw message text, forwarding any arguments that follow the command.
func buildFullCommand(cmdName, messageText string) string {
	args := parseCommandArgs(messageText)
	return "/" + cmdName + joinArgs(args, " ")
}

// buildSkillCommand constructs a skill invocation string from a command name and
// the raw message text, converting /skill args to "Use skill skill. args".
func buildSkillCommand(cmdName, messageText string) string {
	args := parseCommandArgs(messageText)
	return "Use " + cmdName + " skill" + joinArgs(args, ". ")
}

// transcribeVoiceMessage downloads a Telegram voice message and transcribes it via the voice package.
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

// downloadTelegramFile downloads a file from Telegram and saves it to the team/agent file directory.
// Returns the local file path.
func downloadTelegramFile(
	ctx context.Context, b *bot.Bot,
	fileID, teamName, agentName, filename string,
) (string, error) {
	// Sanitize all path components to prevent path traversal
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

	// Avoid overwriting existing files — append timestamp
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

// handleCallbackQuery processes inline keyboard button presses.
func handleCallbackQuery(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery,
	chatID int64, ahs *askHumanStore,
) {
	// Always acknowledge the callback
	defer func() {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}()

	// Validate chat ID — MaybeInaccessibleMessage is a struct, check Type
	if cq.Message.Type != models.MaybeInaccessibleMessageTypeMessage || cq.Message.Message == nil {
		return
	}
	msg := cq.Message.Message
	if msg.Chat.ID != chatID {
		return
	}

	data := cq.Data
	if strings.HasPrefix(data, "ah:") {
		handleAskHumanCallback(ctx, b, cq, data, ahs)
	}
}

// handleAskHumanCallback handles ah:<shortID>:<optIdxOrSkip> callback data.
func handleAskHumanCallback(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery,
	data string, ahs *askHumanStore,
) {
	// Parse: ah:<shortID>:<optIdx> or ah:<shortID>:skip
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return
	}
	shortID := parts[1]
	action := parts[2]

	if action == "skip" {
		if ahs.deliverSkip(shortID) {
			_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
				ChatID:    cq.Message.Message.Chat.ID,
				MessageID: cq.Message.Message.ID,
				Text:      "⏭ Question skipped",
				ParseMode: models.ParseModeHTML,
			})
		} else {
			answerExpiredCallback(ctx, b, cq)
		}
		return
	}

	optIdx, err := strconv.Atoi(action)
	if err != nil {
		return
	}

	// Get entry before delivering (entry is removed on deliver).
	e, ok := ahs.get(shortID)
	if !ok {
		answerExpiredCallback(ctx, b, cq)
		return
	}
	if optIdx < 0 || optIdx >= len(e.options) {
		return
	}

	answer := e.options[optIdx]
	chatID := e.chatID
	msgID := e.msgID
	if ahs.deliverAnswer(shortID, answer) {
		answeredText := fmt.Sprintf("✅ Answered: <b>%s</b>", answer)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msgID,
			Text:      answeredText,
			ParseMode: models.ParseModeHTML,
		})
	}
}

// answerExpiredCallback tells the user via callback alert that the question has expired.
func answerExpiredCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "This question has expired.",
		ShowAlert:       true,
	})
}

// interceptedAsHumanAnswer checks if a text message is an answer to a pending no-option
// ask-human question. Returns true if consumed (should not be forwarded to the agent).
func interceptedAsHumanAnswer(msg *models.Message, ahs *askHumanStore) bool {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return false
	}

	shortID, e, ok := ahs.getForChat(msg.Chat.ID)
	if !ok {
		return false
	}

	if ahs.deliverAnswer(shortID, text) {
		answeredText := fmt.Sprintf("✅ Answered: <b>%s</b>", text)
		editAskHumanMessage(e.botToken, e.chatID, e.msgID, answeredText, nil)
	}
	return true
}

// persistInbound logs a warning if the inbound message cannot be persisted.
func persistInbound(msgSvc *message.Service, sender, recipient, team, content string) {
	if _, err := msgSvc.Create(context.Background(), message.CreateParams{
		Sender: sender, Recipient: recipient, Content: content,
		Team: team, Channel: message.ChannelTelegram,
	}); err != nil {
		log.Printf("[telegram] message persist failed (sender=%s): %v", sender, err)
	}
}

// startTelegramPollers deduplicates agents by bot token and starts one poller per token.
func startTelegramPollers(
	mcfg *config.DaemonConfig, registry *adapterRegistry, done chan struct{},
	ahs *askHumanStore, allCommands []BotCommand,
	mt *messageTracker, msgSvc *message.Service,
) {
	allAgents := mcfg.AllAgents()
	tokenTargets := buildTokenTargets(allAgents)

	for botToken, targets := range tokenTargets {
		dispatchMap := buildDispatchMap(targets)
		log.Printf("[daemon] starting multi-agent poller for %d agents on token ...%s",
			len(targets), botToken[len(botToken)-min(4, len(botToken)):])
		startMultiAgentPoller(botToken, dispatchMap, func(teamName, agentName, text string) {
			if err := deliverToAgent(registry, mcfg, teamName, agentName, text); err != nil {
				log.Printf("[daemon] agent delivery failed for %s: %v", agentName, err)
			}
		}, done, ahs, allCommands, mt, msgSvc,
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

// discoverAndRegisterCommands discovers dynamic commands and registers them with Telegram bots.
func discoverAndRegisterCommands(mcfg *config.DaemonConfig) []BotCommand {
	allAgents := mcfg.AllAgents()
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

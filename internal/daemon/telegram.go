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
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/config"
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
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
	allCommands []BotCommand, mt *messageTracker,
) {
	go func() {
		backoff := 2 * time.Second

		for {
			select {
			case <-done:
				return
			default:
			}

			if err := runMultiAgentPoller(botToken, dispatch, onMessage, done, qs, cas, registry, allCommands, mt); err != nil {
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
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
	allCommands []BotCommand, mt *messageTracker,
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
		handleDefaultUpdate(ctx, b, update, dispatch, botToken, botUsername, onMessage, qs, cas, registry, mt)
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
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
	mt *messageTracker,
) {
	if update.CallbackQuery != nil {
		if update.CallbackQuery.Message.Type == models.MaybeInaccessibleMessageTypeMessage &&
			update.CallbackQuery.Message.Message != nil {
			chatID := update.CallbackQuery.Message.Message.Chat.ID
			if _, ok := dispatch[chatID]; ok {
				handleCallbackQuery(ctx, b, update.CallbackQuery, chatID, qs, cas, registry)
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
			if interceptedAsCustomAnswer(ctx, b, msg, qs, cas, registry) {
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
			mt, &stripped,
		)
	default:
		// DM / private: route by chat ID (existing behaviour, unchanged).
		target, ok := dispatch[msg.Chat.ID]
		if !ok {
			return
		}
		if msg.Text != "" {
			if interceptedAsCustomAnswer(ctx, b, msg, qs, cas, registry) {
				return
			}
		}
		handleInboundMessage(
			ctx, b, msg,
			target.teamName, target.agentName, botToken, target.chatID,
			func(agentName, text string) {
				onMessage(target.teamName, agentName, text)
			},
			mt, nil,
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

	if len(dispatch) > 1 {
		log.Printf("[telegram] WARNING: group message for @%s — multiple agents share token; routing to first match",
			botUsername)
	}
	for _, t := range dispatch {
		target := t
		return &target
	}
	return nil
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
	mt *messageTracker,
	overrideText *string,
) {
	// Track this message for tool reactions
	if mt != nil {
		chatID, err := telegram.ParseChatID(chatIDStr)
		if err == nil {
			mt.set(teamName, agentName, trackedMessage{
				ChatID:    chatID,
				MessageID: msg.ID,
				BotToken:  botToken,
			})
		}
	}

	senderName := msg.From.Username
	if senderName == "" {
		senderName = msg.From.FirstName
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
		text := "[🎤 voice] " + transcription
		formatted := formatInboundMessage(agentName, senderName, replyCtx+text)
		onMessage(agentName, formatted)
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

		text := fmt.Sprintf("[📷 photo] %s", localPath)
		if caption := msg.Caption; caption != "" {
			text += " " + caption
		}
		onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+text))
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

		text := fmt.Sprintf("[📎 file] %s", localPath)
		if caption := msg.Caption; caption != "" {
			text += " " + caption
		}
		onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+text))
		return
	}

	var text string
	if overrideText != nil {
		text = *overrideText
	} else {
		text = strings.TrimSpace(msg.Text)
	}
	onMessage(agentName, formatInboundMessage(agentName, senderName, replyCtx+text))
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
			msg := update.Message
			isGroup := msg.Chat.Type == chatTypeGroup || msg.Chat.Type == chatTypeSupergroup
			// In groups: no chatID-based authorization (group ID ≠ DM chatID).
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
				sendKeysToAgent(teamName, agentName, botToken, chatIDStr, fullCmd,
					fmt.Sprintf("Sent /%s to %s", origName, agentName))
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

// handleCallbackQuery processes inline keyboard button presses for question batches.
func handleCallbackQuery(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery,
	chatID int64, qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
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

	// Clear any pending custom answer state on any button press
	cas.clear(msg.Chat.ID)

	data := cq.Data

	switch {
	case strings.HasPrefix(data, "q:"):
		handleOptionSelect(ctx, b, cq, data, qs, cas, registry)
	case strings.HasPrefix(data, "qnav:"):
		handleNavigation(ctx, b, cq, data, qs, cas)
	case strings.HasPrefix(data, "qsubmit:"):
		handleSubmit(ctx, b, cq, data, qs, registry)
	case strings.HasPrefix(data, "qskip:"):
		handleSkip(ctx, b, cq, data, qs, cas, registry)
	}
}

func handleOptionSelect(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, data string,
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
) {
	// Parse: q:<shortID>:<qIdx>:<optIdx> or q:<shortID>:<qIdx>:custom
	parts := strings.Split(data, ":")
	if len(parts) != 4 {
		return
	}
	shortID := parts[1]
	qIdx, err := strconv.Atoi(parts[2])
	if err != nil {
		return
	}

	batch, ok := qs.get(shortID)
	if !ok {
		answerExpiredCallback(ctx, b, cq)
		return
	}

	// Validate qIdx before any use (including "custom" branch)
	if qIdx < 0 || qIdx >= len(batch.Questions) {
		return
	}

	if parts[3] == "custom" {
		cas.set(cq.Message.Message.Chat.ID, &customAnswerState{
			ShortID:     shortID,
			QuestionIdx: qIdx,
			SetAt:       time.Now(),
		})

		page := buildQuestionPage(batch)
		text, markup := telegram.RenderCustomInputPrompt(page)
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:      batch.ChatID,
			MessageID:   batch.TelegramMsgID,
			Text:        text,
			ParseMode:   models.ParseModeHTML,
			ReplyMarkup: markup,
		})
		return
	}

	optIdx, err := strconv.Atoi(parts[3])
	if err != nil || optIdx < 0 || optIdx >= len(batch.Questions[qIdx].Options) {
		return
	}

	batch.mu.Lock()
	batch.Answers[qIdx] = batch.Questions[qIdx].Options[optIdx].Label
	batch.mu.Unlock()

	// Single question batch: submit immediately
	if len(batch.Questions) == 1 {
		if err := submitBatch(ctx, b, batch, qs, registry); err != nil {
			_ = telegram.SendMessage(batch.BotToken, fmt.Sprintf("%d", batch.ChatID), "Failed to send answer: "+err.Error())
		}
		return
	}

	// Multi-question: auto-advance to next unanswered
	batch.mu.Lock()
	advanceToNextUnanswered(batch)
	batch.mu.Unlock()

	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      batch.ChatID,
		MessageID:   batch.TelegramMsgID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
}

func handleNavigation(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, data string,
	qs *questionStore, cas *customAnswerStore,
) {
	parts := strings.Split(data, ":")
	if len(parts) != 3 {
		return
	}
	shortID := parts[1]
	action := parts[2]

	batch, ok := qs.get(shortID)
	if !ok {
		answerExpiredCallback(ctx, b, cq)
		return
	}

	batch.mu.Lock()
	switch action {
	case "prev":
		if batch.CurrentPage > 0 {
			batch.CurrentPage--
		}
	case "next":
		if batch.CurrentPage < len(batch.Questions)-1 {
			batch.CurrentPage++
		}
	case "cancel_custom":
		cas.clear(cq.Message.Message.Chat.ID)
	}
	batch.mu.Unlock()

	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      batch.ChatID,
		MessageID:   batch.TelegramMsgID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
}

func handleSubmit(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, data string,
	qs *questionStore, registry *adapterRegistry,
) {
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		return
	}
	shortID := parts[1]

	batch, ok := qs.get(shortID)
	if !ok {
		answerExpiredCallback(ctx, b, cq)
		return
	}
	if !batch.AllAnswered() {
		return
	}

	if err := submitBatch(ctx, b, batch, qs, registry); err != nil {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
			Text:            "Failed to send answer: " + err.Error(),
			ShowAlert:       true,
		})
	}
}

func handleSkip(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery, data string,
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
) {
	parts := strings.Split(data, ":")
	if len(parts) != 2 {
		return
	}
	shortID := parts[1]

	batch, ok := qs.get(shortID)
	if !ok {
		answerExpiredCallback(ctx, b, cq)
		return
	}

	cas.clear(cq.Message.Message.Chat.ID)

	if err := cancelQuestion(batch, registry); err != nil {
		log.Printf("[questions] cancel error for %s: %v", batch.AgentName, err)
	}

	skippedText := fmt.Sprintf("⏭ <b>%s</b> — question skipped", batch.AgentName)
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    batch.ChatID,
		MessageID: batch.TelegramMsgID,
		Text:      skippedText,
		ParseMode: models.ParseModeHTML,
	})

	qs.remove(shortID)
	log.Printf("[questions] skipped question for %s batch %s", batch.AgentName, shortID)
}

// submitBatch routes answers to the runtime and updates the Telegram message.
// Returns error so callers can provide appropriate user feedback.
func submitBatch(
	ctx context.Context, b *bot.Bot, batch *QuestionBatch,
	qs *questionStore, registry *adapterRegistry,
) error {
	if err := routeQuestionResponse(batch, registry); err != nil {
		log.Printf("[questions] failed to route response for %s: %v", batch.AgentName, err)
		return err
	}

	questions := make([]string, 0, len(batch.Questions))
	answers := make([]string, 0, len(batch.Questions))
	for i, q := range batch.Questions {
		questions = append(questions, q.Text)
		answers = append(answers, batch.Answers[i])
	}
	summary := telegram.RenderSubmittedSummary(batch.AgentName, questions, answers)
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:    batch.ChatID,
		MessageID: batch.TelegramMsgID,
		Text:      summary,
		ParseMode: models.ParseModeHTML,
	})

	qs.remove(batch.ShortID)
	log.Printf("[questions] submitted answers for %s batch %s", batch.AgentName, batch.ShortID)
	return nil
}

// answerExpiredCallback tells the user via callback alert that the question has expired.
func answerExpiredCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "This question has expired.",
		ShowAlert:       true,
	})
}

// interceptedAsCustomAnswer checks if a text message is a custom answer to a pending question.
// Returns true if the message was consumed as a custom answer and should not be forwarded.
func interceptedAsCustomAnswer(
	ctx context.Context, b *bot.Bot, msg *models.Message,
	qs *questionStore, cas *customAnswerStore, registry *adapterRegistry,
) bool {
	state, ok := cas.getAndClear(msg.Chat.ID)
	if !ok {
		return false
	}

	// Check timeout (2 minutes)
	if time.Since(state.SetAt) >= 2*time.Minute {
		return false
	}

	batch, ok := qs.get(state.ShortID)
	if !ok {
		return false
	}

	customText := strings.TrimSpace(msg.Text)
	if customText == "" {
		return false
	}

	batch.mu.Lock()
	batch.Answers[state.QuestionIdx] = customText
	batch.mu.Unlock()

	// Single question: submit immediately
	if len(batch.Questions) == 1 {
		if err := submitBatch(ctx, b, batch, qs, registry); err != nil {
			_ = telegram.SendMessage(batch.BotToken, fmt.Sprintf("%d", batch.ChatID), "Failed to send answer: "+err.Error())
		}
		return true
	}

	// Multi-question: advance and re-render
	batch.mu.Lock()
	advanceToNextUnanswered(batch)
	batch.mu.Unlock()
	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)
	_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      batch.ChatID,
		MessageID:   batch.TelegramMsgID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return true
}

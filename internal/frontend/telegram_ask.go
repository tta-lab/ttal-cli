package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/telegram"
)

const askHumanTimeout = 5 * time.Minute

// askHumanEntry holds state for one pending ask-human question.
type askHumanEntry struct {
	ch       chan askHumanResult
	options  []string // option labels for resolving callback index → label
	botToken string
	chatID   int64
	msgID    int
	origText string // original HTML message text — preserved for appending answers
}

// askHumanResult is the internal result type for an ask-human operation.
type askHumanResult struct {
	answer  string
	skipped bool
}

// askHumanStore manages pending ask-human channels.
type askHumanStore struct {
	mu          sync.Mutex
	pending     map[string]*askHumanEntry // shortID → entry
	chatPending map[int64]string          // chatID → shortID (no-option text replies only)
	nextID      atomic.Int64
}

func newAskHumanStore() *askHumanStore {
	return &askHumanStore{
		pending:     make(map[string]*askHumanEntry),
		chatPending: make(map[int64]string),
	}
}

func (s *askHumanStore) nextShortID() string {
	return fmt.Sprintf("ah%06x", s.nextID.Add(1))
}

// store registers an entry. noOptions=true causes text replies to be intercepted for this chat.
func (s *askHumanStore) store(entry *askHumanEntry, shortID string, noOptions bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending[shortID] = entry
	if noOptions {
		s.chatPending[entry.chatID] = shortID
	}
}

func (s *askHumanStore) get(shortID string) (*askHumanEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.pending[shortID]
	return e, ok
}

// getAndRemove atomically retrieves and removes an entry.
func (s *askHumanStore) getAndRemove(shortID string) (*askHumanEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.pending[shortID]
	if !ok {
		return nil, false
	}
	delete(s.chatPending, e.chatID)
	delete(s.pending, shortID)
	return e, true
}

// getForChat returns the pending no-option entry for a chat (for text reply routing).
func (s *askHumanStore) getForChat(chatID int64) (string, *askHumanEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	shortID, ok := s.chatPending[chatID]
	if !ok {
		return "", nil, false
	}
	e, eok := s.pending[shortID]
	return shortID, e, eok
}

// remove deletes the entry and its chatPending mapping.
func (s *askHumanStore) remove(shortID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e, ok := s.pending[shortID]; ok {
		delete(s.chatPending, e.chatID)
		delete(s.pending, shortID)
	}
}

// deliverAnswer atomically claims and answers the entry.
func (s *askHumanStore) deliverAnswer(shortID, answer string) bool {
	e, ok := s.getAndRemove(shortID)
	if !ok {
		return false
	}
	e.ch <- askHumanResult{answer: answer}
	return true
}

// deliverSkipWithText atomically claims, skips the entry, and returns its original text.
func (s *askHumanStore) deliverSkipWithText(shortID string) (string, bool) {
	e, ok := s.getAndRemove(shortID)
	if !ok {
		return "", false
	}
	e.ch <- askHumanResult{skipped: true}
	return e.origText, true
}

const telegramMaxLen = 4096

// capText appends suffix to origText, truncating origText on rune boundaries if necessary
// to stay within Telegram's 4096-character limit.
func capText(origText, suffix string) string {
	combined := origText + "\n\n" + suffix
	if utf8.RuneCountInString(combined) <= telegramMaxLen {
		return combined
	}
	overhead := 2 + utf8.RuneCountInString(suffix) + 1 // "\n\n" + suffix + "…"
	maxOrigRunes := telegramMaxLen - overhead
	if maxOrigRunes < 0 {
		return suffix
	}
	runes := []rune(origText)
	if len(runes) > maxOrigRunes {
		runes = runes[:maxOrigRunes]
	}
	return string(runes) + "…\n\n" + suffix
}

// AskHumanHTTPHandler returns an http.HandlerFunc for POST /ask/human.
// daemon.go wires this into httpHandlers.askHuman.
func (f *TelegramFrontend) AskHumanHTTPHandler() http.HandlerFunc {
	return handleHTTPAskHuman(f.ahs, f.cfg.MCfg)
}

// AskHuman implements the direct server-side ask-human call.
func (f *TelegramFrontend) AskHuman(
	_ context.Context, agentName, question string, options []string,
) (string, bool, error) {
	ta := f.findAgent(agentName)
	if ta == nil {
		return "", false, fmt.Errorf("agent %q not found", agentName)
	}
	botToken := config.AgentBotToken(agentName)
	if botToken == "" {
		return "", false, fmt.Errorf("no bot token for agent %q", agentName)
	}
	chatID, err := telegram.ParseChatID(ta.ChatID)
	if err != nil {
		return "", false, fmt.Errorf("invalid chat ID for agent %q: %w", agentName, err)
	}

	shortID := f.ahs.nextShortID()
	noOptions := len(options) == 0
	text, markup := buildAskHumanMessage(agentName, question, options, shortID)
	msgID, err := sendAskHumanMessage(botToken, chatID, text, markup)
	if err != nil {
		return "", false, fmt.Errorf("send telegram message: %w", err)
	}

	ch := make(chan askHumanResult, 1)
	entry := &askHumanEntry{ch: ch, options: options, botToken: botToken, chatID: chatID, msgID: msgID, origText: text}
	f.ahs.store(entry, shortID, noOptions)

	go func() {
		time.Sleep(askHumanTimeout)
		if origText, ok := f.ahs.deliverSkipWithText(shortID); ok {
			expiredText := capText(origText, "→ ⏰ <b>Expired</b> (no response within 5m)")
			editAskHumanMessage(botToken, chatID, msgID, expiredText)
		}
	}()

	result := <-ch
	return result.answer, result.skipped, nil
}

// handleHTTPAskHuman is the HTTP handler for POST /ask/human.
func handleHTTPAskHuman(store *askHumanStore, mcfg *config.DaemonConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req askHumanHTTPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAskHumanJSON(w, http.StatusBadRequest, askHumanHTTPResponse{
				OK: false, Error: "invalid JSON: " + err.Error(),
			})
			return
		}

		botToken, chatID, displayName, err := resolveAskHumanTarget(req, mcfg)
		if err != nil {
			writeAskHumanJSON(w, http.StatusBadRequest, askHumanHTTPResponse{
				OK: false, Error: err.Error(),
			})
			return
		}

		shortID := store.nextShortID()
		noOptions := len(req.Options) == 0

		text, markup := buildAskHumanMessage(displayName, req.Question, req.Options, shortID)
		msgID, err := sendAskHumanMessage(botToken, chatID, text, markup)
		if err != nil {
			writeAskHumanJSON(w, http.StatusInternalServerError, askHumanHTTPResponse{
				OK: false, Error: "failed to send Telegram message: " + err.Error(),
			})
			return
		}

		ch := make(chan askHumanResult, 1)
		entry := &askHumanEntry{
			ch:       ch,
			options:  req.Options,
			botToken: botToken,
			chatID:   chatID,
			msgID:    msgID,
			origText: text,
		}
		store.store(entry, shortID, noOptions)

		go func() {
			time.Sleep(askHumanTimeout)
			if origText, ok := store.deliverSkipWithText(shortID); ok {
				log.Printf("[ask-human] question timed out (shortID=%s, display=%s)", shortID, displayName)
				expiredText := capText(origText, "→ ⏰ <b>Expired</b> (no response within 5m)")
				editAskHumanMessage(botToken, chatID, msgID, expiredText)
			}
		}()

		select {
		case result := <-ch:
			resp := askHumanHTTPResponse{OK: !result.skipped, Answer: result.answer, Skipped: result.skipped}
			writeAskHumanJSON(w, http.StatusOK, resp)
		case <-r.Context().Done():
			if e, ok := store.getAndRemove(shortID); ok {
				log.Printf("[ask-human] agent disconnected (shortID=%s, display=%s)", shortID, displayName)
				disconnectedText := capText(e.origText, "→ ⚡ <b>Agent disconnected</b>")
				editAskHumanMessage(botToken, chatID, msgID, disconnectedText)
			}
		}
	}
}

// askHumanHTTPRequest is the decoded request body from POST /ask/human.
// Matches the AskHumanRequest struct in daemon/ask_human.go (same wire format).
type askHumanHTTPRequest struct {
	Question  string   `json:"question"`
	Options   []string `json:"options,omitempty"`
	AgentName string   `json:"agent_name,omitempty"`
	Session   string   `json:"session,omitempty"`
}

// askHumanHTTPResponse is the response body for POST /ask/human.
// Matches the AskHumanResponse struct in daemon/ask_human.go (same wire format).
type askHumanHTTPResponse struct {
	OK      bool   `json:"ok"`
	Answer  string `json:"answer,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

func writeAskHumanJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// resolveAskHumanTarget resolves bot token, chat ID, and display name from the request.
func resolveAskHumanTarget(req askHumanHTTPRequest, mcfg *config.DaemonConfig) (
	botToken string, chatID int64, displayName string, err error,
) {
	if req.AgentName != "" {
		token := config.AgentBotToken(req.AgentName)
		if token == "" {
			return "", 0, "", fmt.Errorf("no bot token for agent %q", req.AgentName)
		}
		ta, ok := mcfg.FindAgent(req.AgentName)
		if !ok {
			return "", 0, "", fmt.Errorf("agent %q not found in config", req.AgentName)
		}
		id, parseErr := telegram.ParseChatID(ta.ChatID)
		if parseErr != nil {
			return "", 0, "", fmt.Errorf("invalid chat ID for agent %q: %w", req.AgentName, parseErr)
		}
		return token, id, req.AgentName, nil
	}

	if req.Session != "" {
		defaultTeam := mcfg.DefaultTeamName()
		team, ok := mcfg.Teams[defaultTeam]
		if !ok || team.NotificationToken == "" {
			return "", 0, "", fmt.Errorf("no notification bot configured for default team")
		}
		id, parseErr := telegram.ParseChatID(team.ChatID)
		if parseErr != nil {
			return "", 0, "", fmt.Errorf("invalid notification chat ID: %w", parseErr)
		}
		return team.NotificationToken, id, req.Session, nil
	}

	return "", 0, "", fmt.Errorf("must set TTAL_AGENT_NAME or be in a tmux session")
}

// buildAskHumanMessage builds the message text and optional inline keyboard.
func buildAskHumanMessage(
	displayName, question string, options []string, shortID string,
) (string, *models.InlineKeyboardMarkup) {
	text := fmt.Sprintf("❓ <b>%s</b> asks:\n%s", displayName, question)

	if len(options) == 0 {
		text += "\n\n💬 Reply to this message with your answer."
		return text, nil
	}

	rows := make([][]models.InlineKeyboardButton, 0, len(options)+1)
	for i, opt := range options {
		cb := fmt.Sprintf("ah:%s:%d", shortID, i)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: opt, CallbackData: cb},
		})
	}
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "❌ Skip", CallbackData: fmt.Sprintf("ah:%s:skip", shortID)},
	})
	return text, &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// newAskHumanBot creates a bot instance and a 10-second context for a single API call.
func newAskHumanBot(botToken string) (*bot.Bot, context.Context, context.CancelFunc, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	b, err := bot.New(botToken)
	if err != nil {
		cancel()
		return nil, nil, nil, fmt.Errorf("bot init: %w", err)
	}
	return b, ctx, cancel, nil
}

// sendAskHumanMessage sends the ask-human Telegram message and returns its message ID.
func sendAskHumanMessage(botToken string, chatID int64, text string, markup *models.InlineKeyboardMarkup) (int, error) {
	b, ctx, cancel, err := newAskHumanBot(botToken)
	if err != nil {
		return 0, err
	}
	defer cancel()

	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if markup != nil {
		params.ReplyMarkup = markup
	}
	msg, err := b.SendMessage(ctx, params)
	if err != nil {
		return 0, err
	}
	return msg.ID, nil
}

// editAskHumanMessage edits the ask-human Telegram message in-place.
func editAskHumanMessage(botToken string, chatID int64, msgID int, text string) {
	b, ctx, cancel, err := newAskHumanBot(botToken)
	if err != nil {
		log.Printf("[ask-human] bot init error: %v", err)
		return
	}
	defer cancel()

	params := &bot.EditMessageTextParams{
		ChatID:    chatID,
		MessageID: msgID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if _, err := b.EditMessageText(ctx, params); err != nil {
		log.Printf("[ask-human] edit message error: %v", err)
	}
}

// handleCallbackQuery processes inline keyboard button presses.
func handleCallbackQuery(
	ctx context.Context, b *bot.Bot, cq *models.CallbackQuery,
	chatID int64, ahs *askHumanStore,
) {
	defer func() {
		_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
			CallbackQueryID: cq.ID,
		})
	}()

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
	parts := strings.SplitN(data, ":", 3)
	if len(parts) != 3 {
		return
	}
	shortID := parts[1]
	action := parts[2]

	if action == "skip" {
		origText, ok := ahs.deliverSkipWithText(shortID)
		if !ok {
			answerExpiredCallback(ctx, b, cq)
			return
		}
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    cq.Message.Message.Chat.ID,
			MessageID: cq.Message.Message.ID,
			Text:      capText(origText, "→ ⏭ <b>Skipped</b>"),
			ParseMode: models.ParseModeHTML,
		})
		return
	}

	optIdx := 0
	if _, err := fmt.Sscanf(action, "%d", &optIdx); err != nil {
		return
	}

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
	origText := e.origText // capture before deliverAnswer removes the entry
	if ahs.deliverAnswer(shortID, answer) {
		answeredText := capText(origText, fmt.Sprintf("→ <b>%s</b>", answer))
		_, _ = b.EditMessageText(ctx, &bot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msgID,
			Text:      answeredText,
			ParseMode: models.ParseModeHTML,
		})
	} else {
		answerExpiredCallback(ctx, b, cq)
	}
}

// answerExpiredCallback tells the user the question has expired.
func answerExpiredCallback(ctx context.Context, b *bot.Bot, cq *models.CallbackQuery) {
	_, _ = b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: cq.ID,
		Text:            "This question has expired.",
		ShowAlert:       true,
	})
}

// interceptedAsHumanAnswer checks if a text message is an answer to a pending no-option
// ask-human question. Returns true if consumed.
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
		answeredText := capText(e.origText, fmt.Sprintf("→ 💬 <b>%s</b>", text))
		editAskHumanMessage(e.botToken, e.chatID, e.msgID, answeredText)
		return true
	}
	log.Printf("[ask-human] text reply delivery failed for shortID=%s (already answered/timed out)", shortID)
	return false
}

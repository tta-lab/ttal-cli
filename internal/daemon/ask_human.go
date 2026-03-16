package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/telegram"
)

const askHumanTimeout = 5 * time.Minute

// AskHumanRequest is the CLI → daemon POST body for POST /ask/human.
type AskHumanRequest struct {
	Question  string   `json:"question"`
	Options   []string `json:"options,omitempty"`
	AgentName string   `json:"agent_name,omitempty"` // from TTAL_AGENT_NAME
	Session   string   `json:"session,omitempty"`    // from tmux session name
}

// AskHumanResponse is the daemon → CLI JSON response for /ask/human.
type AskHumanResponse struct {
	OK      bool   `json:"ok"`
	Answer  string `json:"answer,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// askHumanEntry holds state for one pending ask-human question.
type askHumanEntry struct {
	ch       chan AskHumanResponse
	options  []string // option labels, for resolving callback index → label
	botToken string
	chatID   int64
	msgID    int
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

// getAndRemove atomically retrieves and removes an entry, preventing TOCTOU races
// between concurrent callers (e.g. timeout goroutine vs. button press).
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
// Returns false if the entry is not found (already answered or timed out).
func (s *askHumanStore) deliverAnswer(shortID, answer string) bool {
	e, ok := s.getAndRemove(shortID)
	if !ok {
		return false
	}
	e.ch <- AskHumanResponse{OK: true, Answer: answer}
	return true
}

// deliverSkip atomically claims and skips the entry.
// Returns false if the entry is not found (already answered or timed out).
func (s *askHumanStore) deliverSkip(shortID string) bool {
	e, ok := s.getAndRemove(shortID)
	if !ok {
		return false
	}
	e.ch <- AskHumanResponse{OK: false, Skipped: true}
	return true
}

// handleHTTPAskHuman is the HTTP handler for POST /ask/human.
func handleHTTPAskHuman(store *askHumanStore, mcfg *config.DaemonConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req AskHumanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, AskHumanResponse{
				OK: false, Error: "invalid JSON: " + err.Error(),
			})
			return
		}

		botToken, chatID, displayName, err := resolveAskHumanTarget(req, mcfg)
		if err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, AskHumanResponse{
				OK: false, Error: err.Error(),
			})
			return
		}

		shortID := store.nextShortID()
		noOptions := len(req.Options) == 0

		text, markup := buildAskHumanMessage(displayName, req.Question, req.Options, shortID)
		msgID, err := sendAskHumanMessage(botToken, chatID, text, markup)
		if err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AskHumanResponse{
				OK: false, Error: "failed to send Telegram message: " + err.Error(),
			})
			return
		}

		ch := make(chan AskHumanResponse, 1)
		entry := &askHumanEntry{
			ch:       ch,
			options:  req.Options,
			botToken: botToken,
			chatID:   chatID,
			msgID:    msgID,
		}
		store.store(entry, shortID, noOptions)

		// Timeout goroutine — fires after 5m if not answered.
		go func() {
			time.Sleep(askHumanTimeout)
			if store.deliverSkip(shortID) {
				log.Printf("[ask-human] question timed out (shortID=%s, display=%s)", shortID, displayName)
				expiredText := fmt.Sprintf("⏰ <b>%s</b> — question expired (no response within 5m)", displayName)
				editAskHumanMessage(botToken, chatID, msgID, expiredText, nil)
			}
		}()

		// Block until answered, skipped (timeout), or client disconnect.
		select {
		case resp := <-ch:
			writeHTTPJSON(w, http.StatusOK, resp)
		case <-r.Context().Done():
			store.remove(shortID)
			log.Printf("[ask-human] agent disconnected (shortID=%s, display=%s)", shortID, displayName)
			disconnectedText := fmt.Sprintf("⚡ <b>%s</b> — agent disconnected before receiving answer", displayName)
			editAskHumanMessage(botToken, chatID, msgID, disconnectedText, nil)
		}
	}
}

// resolveAskHumanTarget resolves bot token, chat ID, and display name from the request.
func resolveAskHumanTarget(req AskHumanRequest, mcfg *config.DaemonConfig) ( //nolint:lll
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
func buildAskHumanMessage( //nolint:lll
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

// sendAskHumanMessage sends the ask-human Telegram message and returns its ID.
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
func editAskHumanMessage(botToken string, chatID int64, msgID int, text string, markup *models.InlineKeyboardMarkup) {
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
	if markup != nil {
		params.ReplyMarkup = markup
	}

	if _, err := b.EditMessageText(ctx, params); err != nil {
		log.Printf("[ask-human] edit message error: %v", err)
	}
}

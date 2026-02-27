package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	cx "codeberg.org/clawteam/ttal-cli/internal/runtime/codex"
	oc "codeberg.org/clawteam/ttal-cli/internal/runtime/opencode"
	"codeberg.org/clawteam/ttal-cli/internal/telegram"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// QuestionBatch holds state for a pending multi-question interaction.
// All mutable fields (Answers, CurrentPage, TelegramMsgID) are protected by mu.
type QuestionBatch struct {
	mu            sync.Mutex
	ShortID       string
	CorrelationID string
	AgentName     string
	Runtime       runtime.Runtime
	Questions     []runtime.Question
	Answers       map[int]string // questionIndex → selected answer text
	CurrentPage   int            // 0-indexed
	TelegramMsgID int
	ChatID        int64
	BotToken      string
	CreatedAt     time.Time
}

// AllAnswered returns true if every question has an answer.
func (b *QuestionBatch) AllAnswered() bool {
	return len(b.Answers) >= len(b.Questions)
}

// questionStore manages pending question batches.
type questionStore struct {
	batches sync.Map
	nextID  atomic.Int64
}

func newQuestionStore() *questionStore {
	return &questionStore{}
}

// nextShortID generates a unique short ID without storing the batch.
// Call store() to make the batch visible to callback handlers.
func (s *questionStore) nextShortID() string {
	return fmt.Sprintf("%06x", s.nextID.Add(1))
}

// store registers a batch (whose ShortID was previously assigned via nextShortID).
func (s *questionStore) store(batch *QuestionBatch) {
	if batch.ShortID == "" {
		log.Printf("[questions] BUG: store() called with empty ShortID, skipping")
		return
	}
	s.batches.Store(batch.ShortID, batch)
}

func (s *questionStore) get(shortID string) (*QuestionBatch, bool) {
	v, ok := s.batches.Load(shortID)
	if !ok {
		return nil, false
	}
	return v.(*QuestionBatch), true
}

func (s *questionStore) remove(shortID string) {
	s.batches.Delete(shortID)
}

func (s *questionStore) cleanup(maxAge time.Duration) {
	now := time.Now()
	s.batches.Range(func(key, value interface{}) bool {
		batch := value.(*QuestionBatch)
		if now.Sub(batch.CreatedAt) > maxAge {
			// Notify user the question expired before deleting
			if batch.TelegramMsgID != 0 {
				expiredText := fmt.Sprintf("⏰ Question from <b>%s</b> expired (no response within %s)", batch.AgentName, maxAge.Truncate(time.Minute))
				_ = telegram.EditQuestionMessage(batch.BotToken, batch.ChatID, batch.TelegramMsgID, expiredText, nil)
			}
			log.Printf("[questions] expired batch %s for %s", key, batch.AgentName)
			s.batches.Delete(key)
		}
		return true
	})
}

// customAnswerState tracks a chat awaiting a typed custom answer.
type customAnswerState struct {
	ShortID     string
	QuestionIdx int
	SetAt       time.Time
}

// customAnswerStore maps chatID → pending custom answer state.
type customAnswerStore struct {
	mu    sync.Mutex
	state map[int64]*customAnswerState
}

func newCustomAnswerStore() *customAnswerStore {
	return &customAnswerStore{state: make(map[int64]*customAnswerState)}
}

func (s *customAnswerStore) set(chatID int64, state *customAnswerState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state[chatID] = state
}

func (s *customAnswerStore) getAndClear(chatID int64) (*customAnswerState, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, ok := s.state[chatID]
	if ok {
		delete(s.state, chatID)
	}
	return state, ok
}

func (s *customAnswerStore) clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.state, chatID)
}

// handleIncomingQuestion creates a QuestionBatch and sends to Telegram.
func handleIncomingQuestion(
	store *questionStore,
	agentName string,
	rt runtime.Runtime,
	correlationID string,
	questions []runtime.Question,
	cfg *config.Config,
) {
	if len(questions) == 0 {
		return
	}

	// Log warning for multi-select questions (not yet supported in Telegram UI)
	for _, q := range questions {
		if q.MultiSelect {
			log.Printf("[questions] warning: multi-select not supported in Telegram UI for %s question %q — treating as single-select", agentName, q.Header)
		}
	}

	agentCfg, ok := cfg.Agents[agentName]
	if !ok || agentCfg.BotToken == "" {
		log.Printf("[questions] no bot config for agent %s, dropping question", agentName)
		return
	}
	chatID, err := telegram.ParseChatID(cfg.AgentChatID(agentName))
	if err != nil {
		log.Printf("[questions] invalid chat ID for %s: %v", agentName, err)
		return
	}

	batch := &QuestionBatch{
		ShortID:       store.nextShortID(),
		CorrelationID: correlationID,
		AgentName:     agentName,
		Runtime:       rt,
		Questions:     questions,
		Answers:       make(map[int]string),
		CurrentPage:   0,
		ChatID:        chatID,
		BotToken:      agentCfg.BotToken,
		CreatedAt:     time.Now(),
	}

	page := buildQuestionPage(batch)
	text, markup := telegram.RenderQuestionPage(page)

	// Send to Telegram before registering in store so the batch is fully
	// initialized (including TelegramMsgID) when visible to callback handlers.
	msgID, err := telegram.SendQuestionMessage(agentCfg.BotToken, chatID, text, markup)
	if err != nil {
		log.Printf("[questions] failed to send question to Telegram for %s: %v", agentName, err)
		return
	}
	batch.TelegramMsgID = msgID

	store.store(batch)
	log.Printf("[questions] sent question %s for %s (batch %s)", correlationID, agentName, batch.ShortID)
}

// buildQuestionPage converts a QuestionBatch into a QuestionPage for rendering.
func buildQuestionPage(batch *QuestionBatch) telegram.QuestionPage {
	q := batch.Questions[batch.CurrentPage]
	selectedAnswer := batch.Answers[batch.CurrentPage]

	var options []telegram.QuestionPageOption
	for _, opt := range q.Options {
		options = append(options, telegram.QuestionPageOption{
			Label:       opt.Label,
			Description: opt.Description,
			Selected:    selectedAnswer == opt.Label,
		})
	}

	return telegram.QuestionPage{
		AgentName:      batch.AgentName,
		PageNum:        batch.CurrentPage + 1,
		TotalPages:     len(batch.Questions),
		Header:         q.Header,
		Text:           q.Text,
		Options:        options,
		AllowCustom:    q.AllowCustom,
		SelectedAnswer: selectedAnswer,
		AllAnswered:    batch.AllAnswered(),
		CallbackPrefix: batch.ShortID,
		QuestionIdx:    batch.CurrentPage,
	}
}

// routeQuestionResponse sends collected answers back to the agent's runtime.
func routeQuestionResponse(batch *QuestionBatch, registry *adapterRegistry) error {
	switch batch.Runtime {
	case runtime.ClaudeCode:
		return routeCCResponse(batch)
	case runtime.OpenCode:
		return routeOCResponse(batch, registry)
	case runtime.Codex:
		return routeCodexResponse(batch, registry)
	default:
		return fmt.Errorf("unknown runtime %s for question response", batch.Runtime)
	}
}

// routeCCResponse sends the answer via tmux send-keys.
func routeCCResponse(batch *QuestionBatch) error {
	session := config.AgentSessionName(batch.AgentName)

	if len(batch.Questions) == 1 {
		return tmux.SendKeys(session, batch.AgentName, batch.Answers[0])
	}

	// Multi-question: send JSON answer mapping
	answerMap := make(map[string]string)
	for i, q := range batch.Questions {
		answerMap[q.Text] = batch.Answers[i]
	}
	answerJSON, err := json.Marshal(map[string]interface{}{"answers": answerMap})
	if err != nil {
		return fmt.Errorf("marshal CC question answers: %w", err)
	}
	return tmux.SendKeys(session, batch.AgentName, string(answerJSON))
}

func routeOCResponse(batch *QuestionBatch, registry *adapterRegistry) error {
	adapter, ok := registry.get(batch.AgentName)
	if !ok {
		return fmt.Errorf("no adapter for OC agent %s", batch.AgentName)
	}
	ocAdapter, ok := adapter.(*oc.Adapter)
	if !ok {
		return fmt.Errorf("adapter for %s is not OpenCode", batch.AgentName)
	}

	answers := make([]string, len(batch.Questions))
	for i := range batch.Questions {
		answers[i] = batch.Answers[i]
	}
	return ocAdapter.ReplyToQuestion(context.Background(), batch.CorrelationID, answers)
}

func routeCodexResponse(batch *QuestionBatch, registry *adapterRegistry) error {
	adapter, ok := registry.get(batch.AgentName)
	if !ok {
		return fmt.Errorf("no adapter for Codex agent %s", batch.AgentName)
	}
	cxAdapter, ok := adapter.(*cx.Adapter)
	if !ok {
		return fmt.Errorf("adapter for %s is not Codex", batch.AgentName)
	}

	var answers []runtime.QuestionAnswer
	for i, q := range batch.Questions {
		answers = append(answers, runtime.QuestionAnswer{
			QuestionID: q.ID,
			Answer:     batch.Answers[i],
		})
	}
	return cxAdapter.RespondToUserInput(batch.CorrelationID, answers)
}

// advanceToNextUnanswered moves CurrentPage to the next unanswered question.
func advanceToNextUnanswered(batch *QuestionBatch) {
	for i := batch.CurrentPage + 1; i < len(batch.Questions); i++ {
		if _, answered := batch.Answers[i]; !answered {
			batch.CurrentPage = i
			return
		}
	}
	for i := 0; i < batch.CurrentPage; i++ {
		if _, answered := batch.Answers[i]; !answered {
			batch.CurrentPage = i
			return
		}
	}
	// All answered — stay on last question (Submit button visible)
	batch.CurrentPage = len(batch.Questions) - 1
}

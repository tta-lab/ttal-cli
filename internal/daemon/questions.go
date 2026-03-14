package daemon

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/telegram"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// Timing constants for CC TUI interaction.
const (
	ccOtherInputDelay    = 500 * time.Millisecond // wait for "Other" text input to appear
	ccInterQuestionDelay = 1 * time.Second        // wait for next question prompt to render
)

// QuestionBatch holds state for a pending multi-question interaction.
// All mutable fields (Answers, CurrentPage, TelegramMsgID) are protected by mu.
type QuestionBatch struct {
	mu            sync.Mutex
	ShortID       string
	CorrelationID string
	TeamName      string
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
				expiredText := fmt.Sprintf("⏰ Question from <b>%s</b> expired (no response within %s)",
					batch.AgentName, maxAge.Truncate(time.Minute))
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

// buildQuestionPage converts a QuestionBatch into a QuestionPage for rendering.
func buildQuestionPage(batch *QuestionBatch) telegram.QuestionPage {
	q := batch.Questions[batch.CurrentPage]
	selectedAnswer := batch.Answers[batch.CurrentPage]

	options := make([]telegram.QuestionPageOption, 0, len(q.Options))
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
	default:
		return fmt.Errorf("unknown runtime %s for question response", batch.Runtime)
	}
}

// routeCCResponse sends number keystrokes to CC's TUI select prompt.
// CC's AskUserQuestion accepts digit keys (1-N) for direct option selection;
// the digit after the last option selects "Other" (e.g., if 3 options exist,
// pressing 4 selects "Other").
//
// CC auto-advances to the next question on digit press alone, so we use
// sendDigit (no Enter) for all selections. A final Enter is sent after all
// questions to submit the form.
func routeCCResponse(batch *QuestionBatch) error {
	session := config.AgentSessionName(batch.TeamName, batch.AgentName)
	window := batch.AgentName

	for i, q := range batch.Questions {
		answer := batch.Answers[i]
		optIdx := findOptionIndex(q.Options, answer)

		if optIdx >= 0 {
			// Standard option: digit only (CC auto-advances for multi-question)
			if err := sendDigit(session, window, optIdx+1); err != nil {
				return fmt.Errorf("select option for Q%d: %w", i, err)
			}
		} else {
			// Custom answer: select "Other" then type the text
			if len(q.Options) == 0 {
				return fmt.Errorf("select Other for Q%d: no options", i)
			}
			if err := sendDigit(session, window, len(q.Options)+1); err != nil {
				return fmt.Errorf("select Other for Q%d: %w", i, err)
			}
			time.Sleep(ccOtherInputDelay)
			if err := sendText(session, window, answer); err != nil {
				return fmt.Errorf("type custom answer for Q%d: %w", i, err)
			}
		}

		if i < len(batch.Questions)-1 {
			time.Sleep(ccInterQuestionDelay)
		}
	}

	// Final Enter to submit — needed when last question was a standard
	// option (sendDigit doesn't press Enter). Harmless if last was Other
	// (sendText already pressed Enter).
	if err := tmux.SendRawKey(session, window, "Enter"); err != nil {
		return fmt.Errorf("submit final Enter: %w", err)
	}
	return nil
}

// sendDigit sends a digit keystroke without Enter.
// Used for selecting options — CC processes the digit and auto-advances.
func sendDigit(session, window string, digit int) error {
	if digit < 1 || digit > 9 {
		return fmt.Errorf("digit out of range 1-9: %d", digit)
	}
	return tmux.SendRawKey(session, window, fmt.Sprintf("%d", digit))
}

// sendText types text and presses Enter.
// Used for custom "Other" answers.
func sendText(session, window, text string) error {
	return tmux.SendKeys(session, window, text)
}

// findOptionIndex returns the index of the option with the given label, or -1 for custom answers.
func findOptionIndex(options []runtime.QuestionOption, label string) int {
	for i, opt := range options {
		if opt.Label == label {
			return i
		}
	}
	return -1
}

// cancelQuestion dismisses a pending question without answering.
func cancelQuestion(batch *QuestionBatch, registry *adapterRegistry) error {
	switch batch.Runtime {
	case runtime.ClaudeCode:
		session := config.AgentSessionName(batch.TeamName, batch.AgentName)
		return tmux.SendRawKey(session, batch.AgentName, "Escape")
	default:
		return fmt.Errorf("unknown runtime %s for cancel", batch.Runtime)
	}
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

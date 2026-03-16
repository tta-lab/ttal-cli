package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

// matrixAskEntry holds state for one pending Matrix ask-human question.
type matrixAskEntry struct {
	ch      chan askHumanResult // reuse askHumanResult from telegram_ask.go (same package)
	options []string
	roomID  id.RoomID
	eventID id.EventID // the question message, for editing on timeout
}

// matrixAskStore manages pending ask-human questions for Matrix rooms.
type matrixAskStore struct {
	mu      sync.Mutex
	pending map[id.RoomID]*matrixAskEntry // roomID → entry (one pending question per room)
}

func newMatrixAskStore() *matrixAskStore {
	return &matrixAskStore{pending: make(map[id.RoomID]*matrixAskEntry)}
}

// store registers an entry for the room. If there is an existing pending question,
// it is auto-skipped AFTER releasing the lock to avoid deadlock.
func (s *matrixAskStore) store(roomID id.RoomID, entry *matrixAskEntry) {
	s.mu.Lock()
	old, hadOld := s.pending[roomID]
	s.pending[roomID] = entry
	s.mu.Unlock()
	if hadOld {
		old.ch <- askHumanResult{skipped: true}
	}
}

func (s *matrixAskStore) get(roomID id.RoomID) (*matrixAskEntry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.pending[roomID]
	return e, ok
}

func (s *matrixAskStore) remove(roomID id.RoomID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, roomID)
}

// claimEntry atomically removes and returns the pending entry for a room.
func (s *matrixAskStore) claimEntry(roomID id.RoomID) (*matrixAskEntry, bool) {
	s.mu.Lock()
	e, ok := s.pending[roomID]
	if ok {
		delete(s.pending, roomID)
	}
	s.mu.Unlock()
	return e, ok
}

func (s *matrixAskStore) deliverAnswer(roomID id.RoomID, answer string) bool {
	e, ok := s.claimEntry(roomID)
	if !ok {
		return false
	}
	e.ch <- askHumanResult{answer: answer}
	return true
}

func (s *matrixAskStore) deliverSkip(roomID id.RoomID) bool {
	e, ok := s.claimEntry(roomID)
	if !ok {
		return false
	}
	e.ch <- askHumanResult{skipped: true}
	return true
}

// deliverSkipIfEntry skips only if the entry pointer matches — prevents a stale timeout
// goroutine from skipping a new question that arrived after context cancellation.
func (s *matrixAskStore) deliverSkipIfEntry(roomID id.RoomID, entry *matrixAskEntry) bool {
	s.mu.Lock()
	e, ok := s.pending[roomID]
	if ok && e == entry {
		delete(s.pending, roomID)
	} else {
		ok = false
	}
	s.mu.Unlock()
	if !ok {
		return false
	}
	entry.ch <- askHumanResult{skipped: true}
	return true
}

// AskHuman sends a question to the agent's Matrix room and blocks until answered or timed out.
func (f *MatrixFrontend) AskHuman(
	ctx context.Context, agentName, question string, options []string,
) (string, bool, error) {
	sess, ok := f.sessions[agentName]
	if !ok {
		return "", false, fmt.Errorf("no Matrix session for agent %s", agentName)
	}

	text := buildMatrixAskMessage(agentName, question, options)

	resp, err := sess.client.SendText(ctx, sess.roomID, text)
	if err != nil {
		return "", false, fmt.Errorf("send ask-human message: %w", err)
	}

	ch := make(chan askHumanResult, 1)
	entry := &matrixAskEntry{
		ch:      ch,
		options: options,
		roomID:  sess.roomID,
		eventID: resp.EventID,
	}
	f.mas.store(sess.roomID, entry)

	// Timeout goroutine. Captures entry pointer to guard against skipping a newer question
	// that arrives after context cancellation (same room, new AskHuman call).
	go func() {
		time.Sleep(askHumanTimeout)
		if f.mas.deliverSkipIfEntry(sess.roomID, entry) {
			expiredText := text + "\n\n⏰ _expired (no response within 5m)_"
			if err := editMatrixMessage(sess.client, sess.roomID, resp.EventID, expiredText); err != nil {
				log.Printf("[matrix] ask-human: edit expired failed (agent=%s, event=%s): %v",
					agentName, resp.EventID, err)
			}
		}
	}()

	select {
	case result := <-ch:
		return result.answer, result.skipped, nil
	case <-ctx.Done():
		f.mas.remove(sess.roomID)
		disconnectedText := text + "\n\n⚡ _agent disconnected before receiving answer_"
		if err := editMatrixMessage(sess.client, sess.roomID, resp.EventID, disconnectedText); err != nil {
			log.Printf("[matrix] ask-human: edit disconnect failed (agent=%s, event=%s): %v",
				agentName, resp.EventID, err)
		}
		return "", true, nil // ctx cancel is a normal lifecycle event, not a server fault
	}
}

// buildMatrixAskMessage formats the ask-human question with numbered options.
func buildMatrixAskMessage(agentName, question string, options []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "❓ %s asks:\n%s", agentName, question)
	if len(options) > 0 {
		sb.WriteString("\n")
		for i, opt := range options {
			fmt.Fprintf(&sb, "\n%d. %s", i+1, opt)
		}
		fmt.Fprintf(&sb, "\n\nReply with a number (1-%d), \"skip\", or type your answer.", len(options))
	} else {
		sb.WriteString("\n\nReply with your answer, or \"skip\" to skip.")
	}
	return sb.String()
}

// interceptMatrixAskAnswer checks if a text message is an answer to a pending ask-human
// question in this room. Returns true if consumed, false if no pending question or if the
// timeout already fired (so the message falls through to normal delivery).
func (f *MatrixFrontend) interceptMatrixAskAnswer(roomID id.RoomID, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	if strings.EqualFold(text, "skip") {
		return f.mas.deliverSkip(roomID)
	}

	// Single get — eliminates TOCTOU between existence check and options read.
	entry, ok := f.mas.get(roomID)
	if !ok {
		return false
	}

	if len(entry.options) > 0 {
		var idx int
		if _, err := fmt.Sscanf(text, "%d", &idx); err == nil && idx >= 1 && idx <= len(entry.options) {
			answer := entry.options[idx-1]
			return f.mas.deliverAnswer(roomID, answer)
		}
	}

	return f.mas.deliverAnswer(roomID, text)
}

// editMatrixMessage edits a Matrix message in-place using m.replace relation type.
// Returns an error so callers can log with full context (agent name, event ID).
func editMatrixMessage(client *mautrix.Client, roomID id.RoomID, eventID id.EventID, newText string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	content := &event.MessageEventContent{
		MsgType: event.MsgText,
		Body:    "* " + newText, // fallback for clients that don't support edits
		NewContent: &event.MessageEventContent{
			MsgType: event.MsgText,
			Body:    newText,
		},
		RelatesTo: &event.RelatesTo{
			Type:    event.RelReplace,
			EventID: eventID,
		},
	}
	if _, err := client.SendMessageEvent(ctx, roomID, event.EventMessage, content); err != nil {
		return fmt.Errorf("edit matrix message: %w", err)
	}
	return nil
}

// AskHumanHTTPHandler returns an http.HandlerFunc for POST /ask/human.
func (f *MatrixFrontend) AskHumanHTTPHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req askHumanHTTPRequest // reuse from telegram_ask.go
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeAskHumanJSON(w, http.StatusBadRequest, askHumanHTTPResponse{
				OK: false, Error: "invalid JSON: " + err.Error(),
			})
			return
		}

		if req.AgentName == "" {
			writeAskHumanJSON(w, http.StatusBadRequest, askHumanHTTPResponse{
				OK: false, Error: "agent_name required for Matrix frontend",
			})
			return
		}

		answer, skipped, err := f.AskHuman(r.Context(), req.AgentName, req.Question, req.Options)
		if err != nil {
			writeAskHumanJSON(w, http.StatusInternalServerError, askHumanHTTPResponse{
				OK: false, Error: err.Error(),
			})
			return
		}
		writeAskHumanJSON(w, http.StatusOK, askHumanHTTPResponse{
			OK: !skipped, Answer: answer, Skipped: skipped,
		})
	}
}

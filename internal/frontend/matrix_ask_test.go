package frontend

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// TestBuildMatrixAskMessage_WithOptions verifies message format includes numbered options.
func TestBuildMatrixAskMessage_WithOptions(t *testing.T) {
	msg := buildMatrixAskMessage("yuki", "Which approach?", []string{"Option A", "Option B", "Option C"})
	if !strings.Contains(msg, "yuki asks:") {
		t.Errorf("missing agent name: %q", msg)
	}
	if !strings.Contains(msg, "Which approach?") {
		t.Errorf("missing question: %q", msg)
	}
	if !strings.Contains(msg, "1. Option A") {
		t.Errorf("missing option 1: %q", msg)
	}
	if !strings.Contains(msg, "2. Option B") {
		t.Errorf("missing option 2: %q", msg)
	}
	if !strings.Contains(msg, "3. Option C") {
		t.Errorf("missing option 3: %q", msg)
	}
	if !strings.Contains(msg, "Reply with a number (1-3)") {
		t.Errorf("missing reply instructions: %q", msg)
	}
}

// TestBuildMatrixAskMessage_NoOptions verifies message format includes free-text reply instruction.
func TestBuildMatrixAskMessage_NoOptions(t *testing.T) {
	msg := buildMatrixAskMessage("kestrel", "What is the API URL?", nil)
	if !strings.Contains(msg, "kestrel asks:") {
		t.Errorf("missing agent name: %q", msg)
	}
	if !strings.Contains(msg, "What is the API URL?") {
		t.Errorf("missing question: %q", msg)
	}
	if !strings.Contains(msg, "Reply with your answer, or \"skip\" to skip.") {
		t.Errorf("missing reply instruction: %q", msg)
	}
}

// TestMatrixAskStore_StoreAndDeliver stores an entry and delivers an answer.
func TestMatrixAskStore_StoreAndDeliver(t *testing.T) {
	s := newMatrixAskStore()
	roomID := id.RoomID("!room1:test")
	ch := make(chan askHumanResult, 1)
	s.store(roomID, &matrixAskEntry{ch: ch, roomID: roomID})

	if !s.deliverAnswer(roomID, "my answer") {
		t.Fatal("deliverAnswer returned false, expected true")
	}
	result := <-ch
	if result.answer != "my answer" {
		t.Errorf("expected answer %q, got %q", "my answer", result.answer)
	}
	if result.skipped {
		t.Error("expected skipped=false")
	}
}

// TestMatrixAskStore_DeliverSkip stores an entry and delivers a skip.
func TestMatrixAskStore_DeliverSkip(t *testing.T) {
	s := newMatrixAskStore()
	roomID := id.RoomID("!room2:test")
	ch := make(chan askHumanResult, 1)
	s.store(roomID, &matrixAskEntry{ch: ch, roomID: roomID})

	if !s.deliverSkip(roomID) {
		t.Fatal("deliverSkip returned false, expected true")
	}
	result := <-ch
	if !result.skipped {
		t.Error("expected skipped=true")
	}
}

// TestMatrixAskStore_ReplaceOldQuestion verifies that storing a new entry auto-skips the old one.
func TestMatrixAskStore_ReplaceOldQuestion(t *testing.T) {
	s := newMatrixAskStore()
	roomID := id.RoomID("!room3:test")

	ch1 := make(chan askHumanResult, 1)
	s.store(roomID, &matrixAskEntry{ch: ch1, roomID: roomID})

	ch2 := make(chan askHumanResult, 1)
	s.store(roomID, &matrixAskEntry{ch: ch2, roomID: roomID})

	// Old entry should be auto-skipped.
	result1 := <-ch1
	if !result1.skipped {
		t.Error("expected old question to be auto-skipped")
	}

	// New entry should still be pending.
	if !s.deliverAnswer(roomID, "new answer") {
		t.Fatal("deliverAnswer on new entry returned false")
	}
	result2 := <-ch2
	if result2.answer != "new answer" {
		t.Errorf("expected %q, got %q", "new answer", result2.answer)
	}
}

// TestInterceptMatrixAskAnswer_NumberedOption verifies that a number resolves to the option label.
func TestInterceptMatrixAskAnswer_NumberedOption(t *testing.T) {
	mas := newMatrixAskStore()
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         mas,
	}
	roomID := id.RoomID("!room4:test")
	ch := make(chan askHumanResult, 1)
	mas.store(roomID, &matrixAskEntry{
		ch:      ch,
		roomID:  roomID,
		options: []string{"Alpha", "Beta", "Gamma"},
	})

	if !fe.interceptMatrixAskAnswer(roomID, "2") {
		t.Fatal("interceptMatrixAskAnswer returned false for valid option index")
	}
	result := <-ch
	if result.answer != "Beta" {
		t.Errorf("expected %q, got %q", "Beta", result.answer)
	}
}

// TestInterceptMatrixAskAnswer_Skip verifies that "skip" delivers a skip result.
func TestInterceptMatrixAskAnswer_Skip(t *testing.T) {
	mas := newMatrixAskStore()
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         mas,
	}
	roomID := id.RoomID("!room5:test")
	ch := make(chan askHumanResult, 1)
	mas.store(roomID, &matrixAskEntry{
		ch:     ch,
		roomID: roomID,
	})

	if !fe.interceptMatrixAskAnswer(roomID, "skip") {
		t.Fatal("interceptMatrixAskAnswer returned false for skip")
	}
	result := <-ch
	if !result.skipped {
		t.Error("expected skipped=true")
	}
}

// TestInterceptMatrixAskAnswer_FreeText verifies free-text answers are delivered as-is.
func TestInterceptMatrixAskAnswer_FreeText(t *testing.T) {
	mas := newMatrixAskStore()
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         mas,
	}
	roomID := id.RoomID("!room6:test")
	ch := make(chan askHumanResult, 1)
	mas.store(roomID, &matrixAskEntry{
		ch:      ch,
		roomID:  roomID,
		options: []string{"A", "B"},
	})

	// "99" is out of range for 2 options — should fall through to free-text.
	if !fe.interceptMatrixAskAnswer(roomID, "something entirely different") {
		t.Fatal("interceptMatrixAskAnswer returned false for free-text")
	}
	result := <-ch
	if result.answer != "something entirely different" {
		t.Errorf("expected free-text answer, got %q", result.answer)
	}
}

// TestInterceptMatrixAskAnswer_NoPending verifies false is returned when no question is pending.
func TestInterceptMatrixAskAnswer_NoPending(t *testing.T) {
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         newMatrixAskStore(),
	}
	roomID := id.RoomID("!room7:test")
	if fe.interceptMatrixAskAnswer(roomID, "hello") {
		t.Error("expected false when no pending question, got true")
	}
}

// TestHandleMatrixCommand_Status verifies that an unknown team returns no-data message.
func TestHandleMatrixCommand_Status(t *testing.T) {
	var got string
	replyFn := func(msg string) { got = msg }
	fe := &MatrixFrontend{
		cfg: MatrixConfig{TeamName: "nonexistent-team-xyz"},
	}
	fe.handleMatrixStatusCommand("nonexistent-team-xyz", replyFn, nil)
	if got != "No agent status data available" {
		t.Errorf("expected no-data message, got %q", got)
	}
}

// TestHandleMatrixCommand_Help verifies that /help lists all registered commands.
func TestHandleMatrixCommand_Help(t *testing.T) {
	var got string
	replyFn := func(msg string) { got = msg }
	fe := &MatrixFrontend{
		allCommands: []Command{
			{Name: "status", Description: "Show status"},
			{Name: "help", Description: "List commands"},
		},
	}
	fe.handleMatrixHelpCommand(replyFn)
	if !strings.Contains(got, "/status") {
		t.Errorf("missing /status in help: %q", got)
	}
	if !strings.Contains(got, "/help") {
		t.Errorf("missing /help in help: %q", got)
	}
}

// TestHandleMatrixCommand_Unknown verifies unknown commands produce no reply (silently ignored).
func TestHandleMatrixCommand_Unknown(t *testing.T) {
	var bodies []string
	srv := matrixTestServer(t, &bodies)
	defer srv.Close()

	fe := buildTestFrontend(t, srv, "yuki", "!testroom:test")
	fe.allCommands = []Command{{Name: testCommandStatus, Description: "Show status"}}
	sess := fe.sessions["yuki"]

	fe.handleMatrixCommand("yuki", "/nonexistent-command-xyz", sess.client, sess.roomID)

	if len(bodies) != 0 {
		t.Errorf("expected no reply for unknown command, got %d send requests", len(bodies))
	}
}

// TestInterceptMatrixAskAnswer_TimeoutRace verifies that if the timeout fires first,
// a subsequent interceptMatrixAskAnswer returns false (message falls through to normal delivery).
func TestInterceptMatrixAskAnswer_TimeoutRace(t *testing.T) {
	mas := newMatrixAskStore()
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         mas,
	}
	roomID := id.RoomID("!room8:test")
	ch := make(chan askHumanResult, 1)
	mas.store(roomID, &matrixAskEntry{
		ch:     ch,
		roomID: roomID,
	})

	// Simulate timeout firing: deliverSkip removes the entry.
	if !mas.deliverSkip(roomID) {
		t.Fatal("deliverSkip failed unexpectedly")
	}
	<-ch // drain the skip result

	// Now intercept should return false (no pending entry).
	if fe.interceptMatrixAskAnswer(roomID, "late answer") {
		t.Error("expected false after timeout, got true")
	}
}

// TestAskHumanHTTPHandler_BadJSON verifies that malformed JSON returns 400.
func TestAskHumanHTTPHandler_BadJSON(t *testing.T) {
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         newMatrixAskStore(),
	}
	handler := fe.AskHumanHTTPHandler()

	req := httptest.NewRequest(http.MethodPost, "/ask/human", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "invalid JSON") {
		t.Errorf("expected 'invalid JSON' in body, got %q", body)
	}
}

// TestAskHumanHTTPHandler_MissingAgentName verifies that empty agent_name returns 400.
func TestAskHumanHTTPHandler_MissingAgentName(t *testing.T) {
	fe := &MatrixFrontend{
		sessions:    make(map[string]agentSession),
		lastEventID: make(map[string]id.EventID),
		mas:         newMatrixAskStore(),
	}
	handler := fe.AskHumanHTTPHandler()

	body := `{"question":"what?","agent_name":""}`
	req := httptest.NewRequest(http.MethodPost, "/ask/human", strings.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "agent_name required") {
		t.Errorf("expected 'agent_name required' in body, got %q", w.Body.String())
	}
}

// TestHandleMatrixUsageCommand_NilFn verifies that nil GetUsageFn returns "not available".
func TestHandleMatrixUsageCommand_NilFn(t *testing.T) {
	var got string
	replyFn := func(msg string) { got = msg }
	fe := &MatrixFrontend{cfg: MatrixConfig{GetUsageFn: nil}}
	fe.handleMatrixUsageCommand(replyFn)
	if got != "Usage data not available" {
		t.Errorf("expected not-available message, got %q", got)
	}
}

// TestHandleMatrixUsageCommand_EmptyString verifies that empty GetUsageFn result returns fetching message.
func TestHandleMatrixUsageCommand_EmptyString(t *testing.T) {
	var got string
	replyFn := func(msg string) { got = msg }
	fe := &MatrixFrontend{cfg: MatrixConfig{GetUsageFn: func() string { return "" }}}
	fe.handleMatrixUsageCommand(replyFn)
	if !strings.Contains(got, "still fetching") {
		t.Errorf("expected fetching message, got %q", got)
	}
}

// TestHandleNotifCommand_RestartNilFn verifies that nil RestartFn sends "not configured" reply.
func TestHandleNotifCommand_RestartNilFn(t *testing.T) {
	var bodies []string
	srv := matrixTestServer(t, &bodies)
	defer srv.Close()

	userID := id.NewUserID("notify", "test")
	nc, err := mautrix.NewClient(srv.URL, userID, "notify-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	fe := &MatrixFrontend{
		cfg: MatrixConfig{
			TeamName:  "testteam",
			RestartFn: nil,
		},
		sessions:     make(map[string]agentSession),
		notifyClient: nc,
		notifyRoom:   id.RoomID("!notifyroom:test"),
		lastEventID:  make(map[string]id.EventID),
	}

	fe.handleNotifCommand("/restart")

	if len(bodies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], "not configured") {
		t.Errorf("expected 'not configured' reply, got %q", bodies[0])
	}
}

// TestHandleNotifCommand_Help verifies /help in notification room lists notif commands.
func TestHandleNotifCommand_Help(t *testing.T) {
	var bodies []string
	srv := matrixTestServer(t, &bodies)
	defer srv.Close()

	userID := id.NewUserID("notify", "test")
	nc, err := mautrix.NewClient(srv.URL, userID, "notify-token")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	fe := &MatrixFrontend{
		cfg:          MatrixConfig{TeamName: "testteam"},
		sessions:     make(map[string]agentSession),
		notifyClient: nc,
		notifyRoom:   id.RoomID("!notifyroom:test"),
		lastEventID:  make(map[string]id.EventID),
	}

	fe.handleNotifCommand("/help")

	if len(bodies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(bodies))
	}
	if !strings.Contains(bodies[0], "restart") {
		t.Errorf("expected 'restart' in help body, got %q", bodies[0])
	}
}

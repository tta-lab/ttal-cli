package frontend

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
)

// --- askHumanStore tests ---

func TestAskHumanStore_StoreAndGet(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	entry := &askHumanEntry{ch: ch, chatID: 42}

	s.store(entry, "ah000001", false)

	got, ok := s.get("ah000001")
	if !ok {
		t.Fatal("expected entry to be found after store")
	}
	if got != entry {
		t.Error("expected same entry pointer")
	}

	_, ok = s.get("ah000002")
	if ok {
		t.Error("expected missing shortID to return false")
	}
}

func TestAskHumanStore_GetAndRemove(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	entry := &askHumanEntry{ch: ch, chatID: 99}
	s.store(entry, "ah000001", false)

	e, ok := s.getAndRemove("ah000001")
	if !ok {
		t.Fatal("expected getAndRemove to succeed")
	}
	if e != entry {
		t.Error("expected same entry pointer")
	}

	// Second call must fail — entry already removed.
	_, ok = s.getAndRemove("ah000001")
	if ok {
		t.Error("expected second getAndRemove to fail (already removed)")
	}
}

func TestAskHumanStore_DeliverAnswer(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	s.store(&askHumanEntry{ch: ch, chatID: 1}, "ah000001", false)

	if !s.deliverAnswer("ah000001", "yes") {
		t.Fatal("expected deliverAnswer to return true")
	}
	resp := <-ch
	if resp.answer != "yes" || resp.skipped {
		t.Errorf("unexpected response: answer=%q skipped=%v", resp.answer, resp.skipped)
	}

	// Second delivery must fail.
	if s.deliverAnswer("ah000001", "yes") {
		t.Error("expected second deliverAnswer to return false")
	}
}

func TestAskHumanStore_DeliverSkip(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	s.store(&askHumanEntry{ch: ch, chatID: 2, origText: "original"}, "ah000002", false)

	if _, ok := s.deliverSkipWithText("ah000002"); !ok {
		t.Fatal("expected deliverSkipWithText to return true")
	}
	resp := <-ch
	if !resp.skipped {
		t.Errorf("unexpected response: answer=%q skipped=%v", resp.answer, resp.skipped)
	}

	if _, ok := s.deliverSkipWithText("ah000002"); ok {
		t.Error("expected second deliverSkipWithText to return false")
	}
}

func TestAskHumanStore_GetForChat_NoOptions(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	entry := &askHumanEntry{ch: ch, chatID: 7}
	s.store(entry, "ah000003", true) // noOptions=true → registers chatPending

	shortID, e, ok := s.getForChat(7)
	if !ok {
		t.Fatal("expected getForChat to find entry")
	}
	if shortID != "ah000003" {
		t.Errorf("expected shortID=ah000003, got %q", shortID)
	}
	if e != entry {
		t.Error("expected same entry pointer")
	}
}

func TestAskHumanStore_GetForChat_WithOptions(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	s.store(&askHumanEntry{ch: ch, chatID: 8}, "ah000004", false) // noOptions=false → NOT registered

	_, _, ok := s.getForChat(8)
	if ok {
		t.Error("expected getForChat to return false when noOptions=false")
	}
}

func TestAskHumanStore_Remove_ClearsChatPending(t *testing.T) {
	s := newAskHumanStore()
	ch := make(chan askHumanResult, 1)
	s.store(&askHumanEntry{ch: ch, chatID: 5}, "ah000005", true)

	s.remove("ah000005")

	_, _, ok := s.getForChat(5)
	if ok {
		t.Error("expected getForChat to return false after remove")
	}
}

// --- buildAskHumanMessage tests ---

func TestBuildAskHumanMessage_NoOptions(t *testing.T) {
	text, markup := buildAskHumanMessage("kestrel", "what should I do?", nil, "ah000001")

	if markup != nil {
		t.Error("expected no inline keyboard for no-options question")
	}
	if !strings.Contains(text, "kestrel") {
		t.Error("expected display name in text")
	}
	if !strings.Contains(text, "what should I do?") {
		t.Error("expected question in text")
	}
	if !strings.Contains(text, "Reply to this message") {
		t.Error("expected reply prompt in text for no-options question")
	}
}

func TestBuildAskHumanMessage_WithOptions(t *testing.T) {
	text, markup := buildAskHumanMessage("kestrel", "approve?", []string{"Yes", "No"}, "ah000001")

	if markup == nil {
		t.Fatal("expected inline keyboard for options question")
		return
	}
	if strings.Contains(text, "Reply to this message") {
		t.Error("no-options reply prompt should not appear when options are set")
	}

	// Should have 2 option rows + 1 skip row.
	if len(markup.InlineKeyboard) != 3 {
		t.Fatalf("expected 3 keyboard rows (2 options + skip), got %d", len(markup.InlineKeyboard))
	}

	// Verify callback data format for first option.
	cb0 := markup.InlineKeyboard[0][0].CallbackData
	if cb0 != "ah:ah000001:0" {
		t.Errorf("expected callback data %q, got %q", "ah:ah000001:0", cb0)
	}
	cb1 := markup.InlineKeyboard[1][0].CallbackData
	if cb1 != "ah:ah000001:1" {
		t.Errorf("expected callback data %q, got %q", "ah:ah000001:1", cb1)
	}

	// Skip button.
	skipCB := markup.InlineKeyboard[2][0].CallbackData
	if skipCB != "ah:ah000001:skip" {
		t.Errorf("expected skip callback %q, got %q", "ah:ah000001:skip", skipCB)
	}
}

// --- resolveAskHumanTarget tests ---

func minimalDaemonConfig() *config.DaemonConfig {
	return &config.DaemonConfig{
		Global: &config.Config{},
		Teams:  map[string]*config.ResolvedTeam{},
	}
}

func TestResolveAskHumanTarget_NoContext(t *testing.T) {
	_, _, _, err := resolveAskHumanTarget(askHumanHTTPRequest{}, minimalDaemonConfig())
	if err == nil {
		t.Fatal("expected error when AgentName and Session are both empty")
	}
	if !strings.Contains(err.Error(), "TTAL_AGENT_NAME") {
		t.Errorf("expected error to mention TTAL_AGENT_NAME, got: %v", err)
	}
}

func TestResolveAskHumanTarget_SessionNoTeam(t *testing.T) {
	_, _, _, err := resolveAskHumanTarget(
		askHumanHTTPRequest{Session: "%1"},
		minimalDaemonConfig(), // no teams configured
	)
	if err == nil {
		t.Fatal("expected error when session set but no notification bot configured")
	}
}

// --- HTTP handler tests ---

func TestHTTPAskHuman_BadJSON(t *testing.T) {
	handler := handleHTTPAskHuman(newAskHumanStore(), minimalDaemonConfig())

	req := httptest.NewRequest(http.MethodPost, "/ask/human", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad JSON, got %d", w.Code)
	}
	var resp askHumanHTTPResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false for bad JSON")
	}
	if resp.Error == "" {
		t.Error("expected non-empty Error for bad JSON")
	}
}

func TestHTTPAskHuman_MissingAgentAndSession(t *testing.T) {
	handler := handleHTTPAskHuman(newAskHumanStore(), minimalDaemonConfig())

	body, _ := json.Marshal(askHumanHTTPRequest{Question: "hello?"}) // no AgentName, no Session
	req := httptest.NewRequest(http.MethodPost, "/ask/human", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when AgentName and Session missing, got %d", w.Code)
	}
	var resp askHumanHTTPResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false")
	}
}

const testGateQuestion = "🔒 Go to <b>Implement</b>\n\n📋 fix something\n📁 ttal · <code>a1b2c3d4</code>"

func TestBuildAskHumanMessage_HTMLPreserved(t *testing.T) {
	// Pre-escaped HTML from askHumanGate — tags should render, not be double-escaped
	text, _ := buildAskHumanMessage("lux", testGateQuestion, []string{"✅ Approve", "❌ Reject"}, "ah000001")

	if strings.Contains(text, "&lt;b&gt;") {
		t.Error("HTML tags should not be double-escaped")
	}
	if !strings.Contains(text, "<b>Implement</b>") {
		t.Error("expected HTML tags preserved in output")
	}
}

func TestBuildAskHumanMessage_RawInputPassesThrough(t *testing.T) {
	// After removing internal escaping, raw input passes through as-is.
	// This verifies the caller is responsible for escaping.
	raw := "<script>alert('xss')</script>"
	text, _ := buildAskHumanMessage("worker", raw, nil, "ah000001")

	// buildAskHumanMessage does NOT escape — raw tags pass through
	if !strings.Contains(text, "<script>") {
		t.Error("expected raw input to pass through unescaped (caller's responsibility)")
	}
}

func TestBuildApprovalText_Approve(t *testing.T) {
	result := buildApprovalText(testGateQuestion, "ignored origText", true)

	if !strings.HasPrefix(result, "✅ <b>Approved</b>\n") {
		t.Error("expected ✅ Approved header")
	}
	if strings.Contains(result, "❓") {
		t.Error("should not contain question mark emoji")
	}
	if strings.Contains(result, "asks:") {
		t.Error("should not contain agent asks prefix")
	}
	if !strings.Contains(result, "🔒 Go to <b>Implement</b>") {
		t.Error("expected question content preserved")
	}
}

func TestBuildApprovalText_Reject(t *testing.T) {
	result := buildApprovalText(testGateQuestion, "ignored origText", false)

	if !strings.HasPrefix(result, "❌ <b>Rejected</b>\n") {
		t.Error("expected ❌ Rejected header")
	}
	if !strings.Contains(result, "🔒 Go to <b>Implement</b>") {
		t.Error("expected question content preserved")
	}
}

func TestBuildApprovalText_EmptyQuestionFallback(t *testing.T) {
	// Backwards compat: in-flight entries created before this change have no question field
	origText := "❓ <b>lux</b> asks:\n🔒 Go to <b>Implement</b>"
	result := buildApprovalText("", origText, true)

	if !strings.HasPrefix(result, "✅ <b>Approved</b>\n") {
		t.Error("expected ✅ Approved header even with fallback")
	}
	if !strings.Contains(result, origText) {
		t.Error("expected origText used as fallback content")
	}
}

func TestApprovalDefaultBranch_NonGateAnswer(t *testing.T) {
	// Non-gate answers should use capText append, not approval format
	origText := "❓ <b>worker</b> asks:\nWhich approach?"
	answer := "Option A"
	result := capText(origText, fmt.Sprintf("→ <b>%s</b>", html.EscapeString(answer)))

	if !strings.Contains(result, "❓") {
		t.Error("non-gate answers should keep original question text")
	}
	if !strings.Contains(result, "→ <b>Option A</b>") {
		t.Error("expected appended answer")
	}
}

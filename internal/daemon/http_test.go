package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// testHandlers returns a minimal httpHandlers for use in router tests.
func testHandlers(sendFn func(SendRequest) error) httpHandlers {
	if sendFn == nil {
		sendFn = func(req SendRequest) error { return nil }
	}
	return httpHandlers{
		send:         sendFn,
		statusUpdate: func(req StatusUpdateRequest) {},
		taskComplete: func(req TaskCompleteRequest) SendResponse { return SendResponse{OK: true} },
	}
}

func TestHTTPSendRoute(t *testing.T) {
	var received SendRequest
	r := newDaemonRouter(testHandlers(func(req SendRequest) error {
		received = req
		return nil
	}))

	body, _ := json.Marshal(SendRequest{To: "kestrel", Message: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.To != "kestrel" {
		t.Errorf("expected To=kestrel, got %q", received.To)
	}
	if received.Message != "hello" {
		t.Errorf("expected Message=hello, got %q", received.Message)
	}
}

func TestHTTPSendRoute_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false for bad JSON")
	}
}

func TestHTTPSendRoute_HandlerError(t *testing.T) {
	r := newDaemonRouter(testHandlers(func(req SendRequest) error {
		return fmt.Errorf("delivery failed")
	}))

	body, _ := json.Marshal(SendRequest{To: "kestrel", Message: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.OK {
		t.Error("expected OK=false on handler error")
	}
}

func TestHTTPGetStatus(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
}

func TestHTTPHealth(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.OK {
		t.Errorf("expected OK=true")
	}
}

func TestHTTPTaskComplete(t *testing.T) {
	var received TaskCompleteRequest
	h := testHandlers(nil)
	h.taskComplete = func(req TaskCompleteRequest) SendResponse {
		received = req
		return SendResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(TaskCompleteRequest{TaskUUID: "abc-123", Team: "default"})
	req := httptest.NewRequest(http.MethodPost, "/task/complete", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.TaskUUID != "abc-123" {
		t.Errorf("expected TaskUUID=abc-123, got %q", received.TaskUUID)
	}
}

func TestHTTPTaskComplete_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/task/complete", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTTPStatusUpdate(t *testing.T) {
	var received StatusUpdateRequest
	h := testHandlers(nil)
	h.statusUpdate = func(req StatusUpdateRequest) { received = req }
	r := newDaemonRouter(h)

	body, _ := json.Marshal(StatusUpdateRequest{Agent: "kestrel", ContextUsedPct: 42.5})
	req := httptest.NewRequest(http.MethodPost, "/status/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Agent != "kestrel" {
		t.Errorf("expected Agent=kestrel, got %q", received.Agent)
	}
}

func TestHTTPStatusUpdate_NilHandler(t *testing.T) {
	h := testHandlers(nil)
	h.statusUpdate = nil
	r := newDaemonRouter(h)

	body, _ := json.Marshal(StatusUpdateRequest{Agent: "kestrel"})
	req := httptest.NewRequest(http.MethodPost, "/status/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// nil statusUpdate handler should still return 200 (no-op)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for nil handler, got %d", w.Code)
	}
}

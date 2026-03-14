package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSendRoute(t *testing.T) {
	var received SendRequest
	r := newDaemonRouter(httpHandlers{
		send: func(req SendRequest) error {
			received = req
			return nil
		},
		statusUpdate: func(req StatusUpdateRequest) {},
		taskComplete: func(req TaskCompleteRequest) SendResponse {
			return SendResponse{OK: true}
		},
	})

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
}

func TestHTTPGetStatus(t *testing.T) {
	r := newDaemonRouter(httpHandlers{
		send:         func(req SendRequest) error { return nil },
		statusUpdate: func(req StatusUpdateRequest) {},
		taskComplete: func(req TaskCompleteRequest) SendResponse { return SendResponse{OK: true} },
	})

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

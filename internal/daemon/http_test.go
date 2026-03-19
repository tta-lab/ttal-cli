package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testAgentName = "kestrel"

// testHandlers returns a minimal httpHandlers for use in router tests.
// All PR handler fields are wired with no-op stubs to avoid nil dereferences
// when tests hit PR routes via newDaemonRouter.
func testHandlers(sendFn func(SendRequest) error) httpHandlers {
	if sendFn == nil {
		sendFn = func(req SendRequest) error { return nil }
	}
	return httpHandlers{
		send:         sendFn,
		statusUpdate: func(req StatusUpdateRequest) {},
		taskComplete: func(req TaskCompleteRequest) SendResponse { return SendResponse{OK: true} },
		breathe:      func(req BreatheRequest) SendResponse { return SendResponse{OK: true} },
		askHuman: func(w http.ResponseWriter, r *http.Request) {
			writeHTTPJSON(w, http.StatusServiceUnavailable, AskHumanResponse{Error: "not configured in test"})
		},
		// PR handlers — always wired to avoid nil dereference when tests exercise the router.
		prCreate:         func(req PRCreateRequest) PRResponse { return PRResponse{OK: true} },
		prModify:         func(req PRModifyRequest) PRResponse { return PRResponse{OK: true} },
		prMerge:          func(req PRMergeRequest) PRResponse { return PRResponse{OK: true} },
		prCheckMergeable: func(req PRCheckMergeableRequest) PRResponse { return PRResponse{OK: true} },
		prCommentCreate:  func(req PRCommentCreateRequest) PRResponse { return PRResponse{OK: true} },
		prCommentList:    func(req PRCommentListRequest) PRResponse { return PRResponse{OK: true} },
		prGetPR: func(req PRGetPRRequest) PRGetPRResponse {
			return PRGetPRResponse{OK: true}
		},
		prGetCombinedStatus: func(req PRGetCombinedStatusRequest) PRCIStatusResponse {
			return PRCIStatusResponse{OK: true}
		},
		prGetCIFailureDetails: func(req PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse {
			return PRCIFailureDetailsResponse{OK: true}
		},
	}
}

func TestHTTPSendRoute(t *testing.T) {
	var received SendRequest
	r := newDaemonRouter(testHandlers(func(req SendRequest) error {
		received = req
		return nil
	}))

	body, _ := json.Marshal(SendRequest{To: testAgentName, Message: "hello"})
	req := httptest.NewRequest(http.MethodPost, "/send", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.To != testAgentName {
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

	body, _ := json.Marshal(SendRequest{To: testAgentName, Message: "hello"})
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

	body, _ := json.Marshal(StatusUpdateRequest{Agent: testAgentName, ContextUsedPct: 42.5})
	req := httptest.NewRequest(http.MethodPost, "/status/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Agent != testAgentName {
		t.Errorf("expected Agent=kestrel, got %q", received.Agent)
	}
}

func TestHTTPBreatheRoute(t *testing.T) {
	var received BreatheRequest
	h := testHandlers(nil)
	h.breathe = func(req BreatheRequest) SendResponse {
		received = req
		return SendResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(BreatheRequest{Agent: testAgentName, Handoff: "# Handoff\n\nNext steps: continue"})
	req := httptest.NewRequest(http.MethodPost, "/breathe", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Agent != testAgentName {
		t.Errorf("expected Agent=kestrel, got %q", received.Agent)
	}
}

func TestHTTPBreatheRoute_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/breathe", bytes.NewReader([]byte("not json")))
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

func TestHTTPBreatheRoute_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.breathe = func(req BreatheRequest) SendResponse {
		return SendResponse{OK: false, Error: "session not found"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(BreatheRequest{Agent: testAgentName, Handoff: "handoff"})
	req := httptest.NewRequest(http.MethodPost, "/breathe", bytes.NewReader(body))
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

func TestHTTPStatusUpdate_NilHandler(t *testing.T) {
	h := testHandlers(nil)
	h.statusUpdate = nil
	r := newDaemonRouter(h)

	body, _ := json.Marshal(StatusUpdateRequest{Agent: testAgentName})
	req := httptest.NewRequest(http.MethodPost, "/status/update", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// nil statusUpdate handler should still return 200 (no-op)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for nil handler, got %d", w.Code)
	}
}

// PR route tests — verify HTTP status codes and request routing for the 9 PR endpoints.

func TestHTTPPRCreate(t *testing.T) {
	var received PRCreateRequest
	h := testHandlers(nil)
	h.prCreate = func(req PRCreateRequest) PRResponse {
		received = req
		return PRResponse{OK: true, PRURL: "https://example.com/pr/1"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRCreateRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Title: "t"})
	req := httptest.NewRequest(http.MethodPost, "/pr/create", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Title != "t" {
		t.Errorf("expected Title=t, got %q", received.Title)
	}
}

func TestHTTPPRCreate_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.prCreate = func(req PRCreateRequest) PRResponse {
		return PRResponse{OK: false, Error: "create failed"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRCreateRequest{ProviderType: "forgejo"})
	req := httptest.NewRequest(http.MethodPost, "/pr/create", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHTTPPRCreate_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))
	req := httptest.NewRequest(http.MethodPost, "/pr/create", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTTPPRGetPR(t *testing.T) {
	h := testHandlers(nil)
	h.prGetPR = func(req PRGetPRRequest) PRGetPRResponse {
		return PRGetPRResponse{OK: true, HeadSHA: "abc123", Merged: false, Mergeable: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetPRRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp PRGetPRResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.HeadSHA != "abc123" {
		t.Errorf("expected HeadSHA=abc123, got %q", resp.HeadSHA)
	}
}

func TestHTTPPRGetPR_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.prGetPR = func(req PRGetPRRequest) PRGetPRResponse {
		return PRGetPRResponse{OK: false, Error: "not found"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetPRRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHTTPPRCIStatus(t *testing.T) {
	h := testHandlers(nil)
	h.prGetCombinedStatus = func(req PRGetCombinedStatusRequest) PRCIStatusResponse {
		return PRCIStatusResponse{OK: true, State: "success"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetCombinedStatusRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", SHA: "abc"})
	req := httptest.NewRequest(http.MethodPost, "/pr/ci/status", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp PRCIStatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.State != "success" {
		t.Errorf("expected State=success, got %q", resp.State)
	}
}

func TestHTTPPRCIStatus_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.prGetCombinedStatus = func(req PRGetCombinedStatusRequest) PRCIStatusResponse {
		return PRCIStatusResponse{OK: false, Error: "fetch failed"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetCombinedStatusRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", SHA: "abc"})
	req := httptest.NewRequest(http.MethodPost, "/pr/ci/status", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHTTPPRCIFailureDetails(t *testing.T) {
	h := testHandlers(nil)
	h.prGetCIFailureDetails = func(req PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse {
		return PRCIFailureDetailsResponse{OK: true, Details: []PRCIFailureDetail{{JobName: "test-job"}}}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetCIFailureDetailsRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", SHA: "abc"})
	req := httptest.NewRequest(http.MethodPost, "/pr/ci/failure-details", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp PRCIFailureDetailsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Details) != 1 || resp.Details[0].JobName != "test-job" {
		t.Errorf("unexpected details: %+v", resp.Details)
	}
}

func TestHTTPPRCIFailureDetails_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.prGetCIFailureDetails = func(req PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse {
		return PRCIFailureDetailsResponse{OK: false, Error: "fetch failed"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRGetCIFailureDetailsRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", SHA: "abc"})
	req := httptest.NewRequest(http.MethodPost, "/pr/ci/failure-details", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

// Smoke tests for remaining PR routes — verify router wiring and error→500 contract.

func TestHTTPPRModify_Smoke(t *testing.T) {
	var received PRModifyRequest
	h := testHandlers(nil)
	h.prModify = func(req PRModifyRequest) PRResponse {
		received = req
		return PRResponse{OK: true}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRModifyRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1, Title: "new"})
	req := httptest.NewRequest(http.MethodPost, "/pr/modify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Title != "new" {
		t.Errorf("expected Title=new, got %q", received.Title)
	}
}

func TestHTTPPRMerge_Smoke(t *testing.T) {
	h := testHandlers(nil)
	h.prMerge = func(req PRMergeRequest) PRResponse { return PRResponse{OK: true} }
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRMergeRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/merge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPPRMerge_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.prMerge = func(req PRMergeRequest) PRResponse { return PRResponse{OK: false, Error: "not mergeable"} }
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRMergeRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/merge", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHTTPPRCheckMergeable_Smoke(t *testing.T) {
	h := testHandlers(nil)
	h.prCheckMergeable = func(req PRCheckMergeableRequest) PRResponse {
		return PRResponse{OK: true, HeadSHA: "abc123"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRCheckMergeableRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/check-mergeable", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPPRCommentCreate_Smoke(t *testing.T) {
	h := testHandlers(nil)
	h.prCommentCreate = func(req PRCommentCreateRequest) PRResponse { return PRResponse{OK: true} }
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRCommentCreateRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1, Body: "LGTM"})
	req := httptest.NewRequest(http.MethodPost, "/pr/comment/create", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHTTPPRCommentList_Smoke(t *testing.T) {
	h := testHandlers(nil)
	h.prCommentList = func(req PRCommentListRequest) PRResponse {
		return PRResponse{OK: true, Comments: []PRCommentItem{{User: "neil", Body: "LGTM"}}}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(PRCommentListRequest{ProviderType: "forgejo", Owner: "o", Repo: "r", Index: 1})
	req := httptest.NewRequest(http.MethodPost, "/pr/comment/list", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp PRResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Comments) != 1 || resp.Comments[0].User != "neil" {
		t.Errorf("unexpected comments: %+v", resp.Comments)
	}
}

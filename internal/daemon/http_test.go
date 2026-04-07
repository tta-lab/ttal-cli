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
		prGetPR: func(req PRGetPRRequest) PRGetPRResponse {
			return PRGetPRResponse{OK: true}
		},
		prGetCombinedStatus: func(req PRGetCombinedStatusRequest) PRCIStatusResponse {
			return PRCIStatusResponse{OK: true}
		},
		prGetCIFailureDetails: func(req PRGetCIFailureDetailsRequest) PRCIFailureDetailsResponse {
			return PRCIFailureDetailsResponse{OK: true}
		},
		pipelineAdvance: func(w http.ResponseWriter, r *http.Request) {
			writeHTTPJSON(w, http.StatusOK, AdvanceResponse{Status: AdvanceStatusNoPipeline, Message: "not configured in test"})
		},
		commentAdd: func(req CommentAddRequest) CommentAddResponse {
			return CommentAddResponse{OK: true, Round: 1}
		},
		commentList: func(req CommentListRequest) CommentListResponse {
			return CommentListResponse{OK: true}
		},
		commentGet: func(req CommentGetRequest) CommentGetResponse {
			return CommentGetResponse{OK: true}
		},
		gitPush: func(req GitPushRequest) GitPushResponse {
			return GitPushResponse{OK: true}
		},
		gitTag: func(req GitTagRequest) GitTagResponse {
			return GitTagResponse{OK: true}
		},
		notify: func(_, _ string) error { return nil },
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

	body, _ := json.Marshal(TaskCompleteRequest{TaskUUID: "abc-123"})
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

// Comment get route tests

func TestHTTPCommentGet(t *testing.T) {
	var received CommentGetRequest
	h := testHandlers(nil)
	h.commentGet = func(req CommentGetRequest) CommentGetResponse {
		received = req
		return CommentGetResponse{OK: true, Comments: []CommentEntry{
			{Author: "reviewer", Body: "looks good", Round: 2, CreatedAt: "2026-03-21T00:00:00Z"},
		}}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(CommentGetRequest{Target: "abc-123", Round: 2})
	req := httptest.NewRequest(http.MethodPost, "/comment/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if received.Target != "abc-123" {
		t.Errorf("expected Target=abc-123, got %q", received.Target)
	}
	if received.Round != 2 {
		t.Errorf("expected Round=2, got %d", received.Round)
	}
	var resp CommentGetResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Comments) != 1 {
		t.Fatalf("want 1 comment, got %d", len(resp.Comments))
	}
	if resp.Comments[0].Author != "reviewer" || resp.Comments[0].Body != "looks good" || resp.Comments[0].Round != 2 {
		t.Errorf("unexpected comment: %+v", resp.Comments[0])
	}
}

func TestHTTPCommentGet_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.commentGet = func(req CommentGetRequest) CommentGetResponse {
		return CommentGetResponse{OK: false, Error: "db error"}
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(CommentGetRequest{Target: "abc-123", Round: 1})
	req := httptest.NewRequest(http.MethodPost, "/comment/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestHTTPCommentGet_ZeroRound(t *testing.T) {
	h := testHandlers(nil)
	h.commentGet = func(req CommentGetRequest) CommentGetResponse {
		return handleCommentGet(nil, "team", req)
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(CommentGetRequest{Target: "abc-123", Round: 0})
	req := httptest.NewRequest(http.MethodPost, "/comment/get", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for round=0, got %d", w.Code)
	}
	var resp CommentGetResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error != "round must be >= 1" {
		t.Errorf("expected round validation error, got %q", resp.Error)
	}
}

func TestHTTPCommentGet_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))
	req := httptest.NewRequest(http.MethodPost, "/comment/get", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
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

func TestHTTPNotify_HappyPath(t *testing.T) {
	var capturedTeam, capturedMsg string
	h := testHandlers(nil)
	h.notify = func(team, msg string) error {
		capturedTeam = team
		capturedMsg = msg
		return nil
	}
	r := newDaemonRouter(h)

	body, _ := json.Marshal(NotifyRequest{Message: "hello from worker"})
	req := httptest.NewRequest(http.MethodPost, "/notify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if capturedTeam != "default" {
		t.Errorf("expected team=default, got %q", capturedTeam)
	}
	if capturedMsg != "hello from worker" {
		t.Errorf("expected msg=hello from worker, got %q", capturedMsg)
	}
}

func TestHTTPNotify_BadJSON(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	req := httptest.NewRequest(http.MethodPost, "/notify", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHTTPNotify_HandlerError(t *testing.T) {
	h := testHandlers(nil)
	h.notify = func(_, _ string) error { return fmt.Errorf("frontend error") }
	r := newDaemonRouter(h)

	body, _ := json.Marshal(NotifyRequest{Message: "test"})
	req := httptest.NewRequest(http.MethodPost, "/notify", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	var resp SendResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Error == "" {
		t.Error("expected error message in response")
	}
}

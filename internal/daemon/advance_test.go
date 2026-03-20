package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestAdvanceRoute_NoPipelineConfigured tests the /pipeline/advance route
// when no pipelines.toml is configured (uses testHandlers stub).
func TestAdvanceRoute_NoPipelineConfigured(t *testing.T) {
	r := newDaemonRouter(testHandlers(nil))

	body, _ := json.Marshal(AdvanceRequest{TaskUUID: "abc12345-1234-1234-1234-123456789abc"})
	req := httptest.NewRequest(http.MethodPost, "/pipeline/advance", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != AdvanceStatusNoPipeline {
		t.Errorf("expected status %q, got %q", AdvanceStatusNoPipeline, resp.Status)
	}
}

// TestAdvanceRoute_CustomHandler tests that a custom pipelineAdvance handler
// is called correctly via the router.
func TestAdvanceRoute_CustomHandler(t *testing.T) {
	var gotReq AdvanceRequest

	h := testHandlers(nil)
	h.pipelineAdvance = func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotReq); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status: AdvanceStatusAdvanced,
			Stage:  "Plan",
		})
	}

	r := newDaemonRouter(h)
	body, _ := json.Marshal(AdvanceRequest{
		TaskUUID:  "abc12345-1234-1234-1234-123456789abc",
		AgentName: "inke",
		Team:      "default",
	})
	req := httptest.NewRequest(http.MethodPost, "/pipeline/advance", bytes.NewReader(body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != AdvanceStatusAdvanced {
		t.Errorf("expected status %q, got %q", AdvanceStatusAdvanced, resp.Status)
	}
	if resp.Stage != "Plan" {
		t.Errorf("expected stage 'Plan', got %q", resp.Stage)
	}
	if gotReq.AgentName != "inke" {
		t.Errorf("expected agent 'inke', got %q", gotReq.AgentName)
	}
}

// TestAdvanceRoute_InvalidJSON tests the /pipeline/advance route with bad input.
func TestAdvanceRoute_InvalidJSON(t *testing.T) {
	h := testHandlers(nil)
	h.pipelineAdvance = func(w http.ResponseWriter, r *http.Request) {
		var req AdvanceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: "invalid JSON: " + err.Error(),
			})
			return
		}
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{Status: AdvanceStatusNoPipeline})
	}

	r := newDaemonRouter(h)
	req := httptest.NewRequest(http.MethodPost, "/pipeline/advance", bytes.NewReader([]byte("not json")))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != AdvanceStatusError {
		t.Errorf("expected status %q, got %q", AdvanceStatusError, resp.Status)
	}
}

// TestFindIdleAgent_NoAgentsForRole tests the error case when no agents have the role.
func TestFindIdleAgent_NoAgentsForRole(t *testing.T) {
	_, err := findIdleAgent("", "nonexistent-role", nil)
	if err == nil {
		t.Error("expected error for nonexistent role, got nil")
	}
}

// TestHasTag verifies the hasTag helper.
func TestHasTag(t *testing.T) {
	tags := []string{"feature", "lgtm", "inke"}

	if !hasTag(tags, "lgtm") {
		t.Error("expected hasTag to find 'lgtm'")
	}
	if hasTag(tags, "hotfix") {
		t.Error("expected hasTag to NOT find 'hotfix'")
	}
	if hasTag(nil, "lgtm") {
		t.Error("expected hasTag to return false for nil tags")
	}
}

// TestFindAgentTag verifies the findAgentTag helper.
func TestFindAgentTag(t *testing.T) {
	agentRoles := map[string]string{
		"inke":   "designer",
		"athena": "researcher",
	}

	got := findAgentTag([]string{"feature", "inke", "lgtm"}, agentRoles)
	if got != "inke" {
		t.Errorf("expected 'inke', got %q", got)
	}

	got = findAgentTag([]string{"feature", "lgtm"}, agentRoles)
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}

	got = findAgentTag(nil, agentRoles)
	if got != "" {
		t.Errorf("expected empty string for nil tags, got %q", got)
	}
}

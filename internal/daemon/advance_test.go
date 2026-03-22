package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testAgentInke = "inke"

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
		AgentName: testAgentInke,
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
	if gotReq.AgentName != testAgentInke {
		t.Errorf("expected agent %q, got %q", testAgentInke, gotReq.AgentName)
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

// TestBuildRouteTrigger_NeverContainsDescription guards against re-introducing task.Description
// into the trigger. Descriptions can contain shell metacharacters (<, >, |, $, ?) that break
// the zsh -c '...' wrapper used to deliver the trigger to agent sessions.
func TestBuildRouteTrigger_NeverContainsDescription(t *testing.T) {
	uuid := "abc12345-1234-1234-1234-123456789abc"
	trigger := buildRouteTrigger(uuid)

	// Must contain the short UUID.
	if !strings.Contains(trigger, "abc12345") {
		t.Errorf("trigger missing UUID: %q", trigger)
	}

	// Must be a single line (no newlines that could smuggle description content).
	if strings.Contains(trigger, "\n") {
		t.Errorf("trigger must be single-line, got: %q", trigger)
	}

	// Must not contain any user-controlled text — only the fixed template and UUID.
	metacharDescriptions := []string{
		"test <foo> bar",
		"worker/<slug>?",
		"it's a task; rm -rf /",
		"pipe | me",
		"$HOME backdoor",
	}
	for _, desc := range metacharDescriptions {
		if strings.Contains(trigger, desc) {
			t.Errorf("trigger must not contain description %q: %q", desc, trigger)
		}
	}
}

// TestFindIdleAgent_NoAgentsForRole tests the error case when no agents have the role.
func TestFindIdleAgent_NoAgentsForRole(t *testing.T) {
	_, err := findIdleAgent("", "nonexistent-role")
	if err == nil {
		t.Error("expected error for nonexistent role, got nil")
	}
}

// TestHasTag verifies the hasTag helper.
func TestHasTag(t *testing.T) {
	tags := []string{"feature", "lgtm", testAgentInke}

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

// TestResolveHintedAgent_HappyPath verifies that a matching idle agent is returned.
func TestResolveHintedAgent_HappyPath(t *testing.T) {
	dir := t.TempDir()
	agentMD := "---\nrole: designer\n---\n# Inke\n"
	if err := os.WriteFile(filepath.Join(dir, testAgentInke+".md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}
	agentRoles := map[string]string{testAgentInke: "designer"}

	orig := countTasksFn
	countTasksFn = func(filters ...string) (int, error) { return 0, nil }
	defer func() { countTasksFn = orig }()

	got := resolveHintedAgent(dir, []string{"brainstorm", testAgentInke}, "designer", agentRoles)
	if got == nil {
		t.Fatal("expected hinted agent, got nil")
	}
	if got.Name != testAgentInke {
		t.Errorf("expected agent name %q, got %q", testAgentInke, got.Name)
	}
}

// TestResolveHintedAgent_BusyFallback verifies nil is returned when hinted agent is busy.
func TestResolveHintedAgent_BusyFallback(t *testing.T) {
	dir := t.TempDir()
	agentMD := "---\nrole: designer\n---\n# Inke\n"
	if err := os.WriteFile(filepath.Join(dir, testAgentInke+".md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}
	agentRoles := map[string]string{testAgentInke: "designer"}

	orig := countTasksFn
	countTasksFn = func(filters ...string) (int, error) { return 1, nil }
	defer func() { countTasksFn = orig }()

	got := resolveHintedAgent(dir, []string{"brainstorm", testAgentInke}, "designer", agentRoles)
	if got != nil {
		t.Errorf("expected nil for busy agent, got %v", got)
	}
}

// TestResolveHintedAgent_WrongRole verifies hints are ignored when role doesn't match.
func TestResolveHintedAgent_WrongRole(t *testing.T) {
	dir := t.TempDir()
	agentRoles := map[string]string{"athena": "researcher"}
	got := resolveHintedAgent(dir, []string{"brainstorm", "athena"}, "designer", agentRoles)
	if got != nil {
		t.Errorf("expected nil for wrong-role hint, got %v", got)
	}
}

// TestResolveHintedAgent_NoHintTag verifies nil when no tag matches an agent.
func TestResolveHintedAgent_NoHintTag(t *testing.T) {
	dir := t.TempDir()
	agentRoles := map[string]string{testAgentInke: "designer"}
	got := resolveHintedAgent(dir, []string{"brainstorm", "feature"}, "designer", agentRoles)
	if got != nil {
		t.Errorf("expected nil when no hint tag, got %v", got)
	}
}

// TestResolveHintedAgent_EmptyTeamPath verifies graceful nil return.
func TestResolveHintedAgent_EmptyTeamPath(t *testing.T) {
	agentRoles := map[string]string{testAgentInke: "designer"}
	got := resolveHintedAgent("", []string{testAgentInke}, "designer", agentRoles)
	if got != nil {
		t.Errorf("expected nil for empty teamPath, got %v", got)
	}
}

// TestFindAgentTag verifies the findAgentTag helper.
func TestFindAgentTag(t *testing.T) {
	agentRoles := map[string]string{
		testAgentInke: "designer",
		"athena":      "researcher",
	}

	got := findAgentTag([]string{"feature", testAgentInke, "lgtm"}, agentRoles)
	if got != testAgentInke {
		t.Errorf("expected %q, got %q", testAgentInke, got)
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

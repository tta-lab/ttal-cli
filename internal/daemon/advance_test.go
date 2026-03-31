package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
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

// TestCheckCallerPastStage_Rejected verifies rejection when caller's stage is already past.
func TestCheckCallerPastStage_Rejected(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	// Caller is "kestrel" with role "fixer" (stage 0), task is at stage 1 (Implement)
	agentRoles := map[string]string{"kestrel": "fixer"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 1, "kestrel", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if !rejected {
		t.Error("expected rejection when caller's stage is past")
	}

	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Status != AdvanceStatusRejected {
		t.Errorf("expected status %q, got %q", AdvanceStatusRejected, resp.Status)
	}
	if !strings.Contains(resp.Message, "Fix") {
		t.Errorf("message should mention caller's stage name: %q", resp.Message)
	}
	if !strings.Contains(resp.Message, "Implement") {
		t.Errorf("message should mention current stage name: %q", resp.Message)
	}
}

// TestCheckCallerPastStage_AllowedSameStage verifies no rejection when caller is at their own stage.
func TestCheckCallerPastStage_AllowedSameStage(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	agentRoles := map[string]string{"kestrel": "fixer"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 0, "kestrel", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if rejected {
		t.Error("should NOT reject when caller is at their own stage")
	}
}

// TestCheckCallerPastStage_AllowedNoAgent verifies no rejection when callerAgent is empty (worker/CLI).
func TestCheckCallerPastStage_AllowedNoAgent(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
		},
	}
	agentRoles := map[string]string{"kestrel": "fixer"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 0, "", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if rejected {
		t.Error("should NOT reject when callerAgent is empty (worker/CLI)")
	}
}

// TestCheckCallerPastStage_AllowedRoleNotInPipeline verifies no rejection when caller has no pipeline stage.
func TestCheckCallerPastStage_AllowedRoleNotInPipeline(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	// Yuki is orchestrator — no matching pipeline stage
	agentRoles := map[string]string{"yuki": "orchestrator"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 1, "yuki", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if rejected {
		t.Error("should NOT reject when caller's role has no pipeline stage")
	}
}

// TestCheckCallerPastStage_AllowedAgentNotInRoles verifies no rejection when caller is not in agentRoles.
func TestCheckCallerPastStage_AllowedAgentNotInRoles(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
		},
	}
	agentRoles := map[string]string{}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 0, "unknown", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if rejected {
		t.Error("should NOT reject when caller is not in agentRoles")
	}
}

// TestCheckCallerPastStage_AllowedFutureStage verifies no rejection when caller's stage is in the future.
func TestCheckCallerPastStage_AllowedFutureStage(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	// Caller role "coder" is at stage 1, task is currently at stage 0 (Fix)
	agentRoles := map[string]string{"worker-agent": "coder"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 0, "worker-agent", agentRoles, "abc12345-1234-1234-1234-123456789abc", nil)
	if rejected {
		t.Error("should NOT reject when caller's stage is in the future")
	}
}

// TestCheckCallerPastStage_AllowedPipelineFullyCompleted verifies no rejection when all stages have LGTM.
func TestCheckCallerPastStage_AllowedPipelineFullyCompleted(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	// All stages have LGTM — CurrentStage returns last stage (idx=1).
	// A fixer calling ttal go should NOT be rejected — let processStageAdvance
	// handle pipeline completion via handlePipelineComplete.
	agentRoles := map[string]string{"kestrel": "fixer"}
	taskTags := []string{"bugfix", "fix", "fix_lgtm", "implement", "implement_lgtm"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 1, "kestrel", agentRoles, "abc12345-1234-1234-1234-123456789abc", taskTags)
	if rejected {
		t.Error("should NOT reject when pipeline is fully completed (all stages have LGTM)")
	}
}

// TestCheckCallerPastStage_AllowedMidPipelineLGTM verifies bypass for a 3-stage pipeline
// where the current (middle) stage already has its LGTM tag but is not the last stage.
// The fixer whose stage (0) is behind the current stage (1) must NOT be rejected —
// the LGTM on the middle stage means processStageAdvance should advance to the next stage.
func TestCheckCallerPastStage_AllowedMidPipelineLGTM(t *testing.T) {
	p := &pipeline.Pipeline{
		Stages: []pipeline.Stage{
			{Name: "Fix", Assignee: "fixer", Gate: "human"},
			{Name: "Review", Assignee: "reviewer", Gate: "human"},
			{Name: "Implement", Assignee: "coder", Gate: "auto"},
		},
	}
	// Fixer (stage 0) calls ttal go; task is at stage 1 (Review) with review_lgtm set.
	agentRoles := map[string]string{"kestrel": "fixer"}
	taskTags := []string{"bugfix", "fix", "fix_lgtm", "review", "review_lgtm"}
	w := httptest.NewRecorder()

	rejected := checkCallerPastStage(w, p, 1, "kestrel", agentRoles, "abc12345-1234-1234-1234-123456789abc", taskTags)
	if rejected {
		t.Error("should NOT reject when current stage already has LGTM (mid-pipeline bypass)")
	}
}

// TestPrependSkills verifies the pipeline.PrependSkills helper (moved from daemon).
func TestPrependSkills(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		skills []string
		rt     runtime.Runtime
		want   string
	}{
		{
			name:   "no skills returns prompt unchanged",
			prompt: "Write a plan",
			skills: nil,
			rt:     runtime.ClaudeCode,
			want:   "Write a plan",
		},
		{
			name:   "empty skills returns prompt unchanged",
			prompt: "Write a plan",
			skills: []string{},
			rt:     runtime.ClaudeCode,
			want:   "Write a plan",
		},
		{
			name:   "single skill prepended CC",
			prompt: "Write a plan",
			skills: []string{"sp-planning"},
			rt:     runtime.ClaudeCode,
			want:   "run ttal skill get sp-planning\n\nWrite a plan",
		},
		{
			name:   "multiple skills prepended CC",
			prompt: "Write a plan",
			skills: []string{"sp-planning", "flicknote"},
			rt:     runtime.ClaudeCode,
			want:   "run ttal skill get sp-planning\nrun ttal skill get flicknote\n\nWrite a plan",
		},
		{
			name:   "codex runtime uses dollar prefix",
			prompt: "Write a plan",
			skills: []string{"sp-planning"},
			rt:     runtime.Codex,
			want:   "$sp-planning\n\nWrite a plan",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pipeline.PrependSkills(tt.prompt, tt.skills, tt.rt)
			if got != tt.want {
				t.Errorf("PrependSkills() =\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

// stubWorktreePathAndNotify overrides worktreePathFn and notifyTelegramFn for testing.
// Returns a cleanup function that restores the originals.
func stubWorktreePathAndNotify(t *testing.T, worktreeDir string) func() {
	t.Helper()
	origPath := worktreePathFn
	worktreePathFn = func(_, _ string) (string, error) { return worktreeDir, nil }
	origNotify := notifyTelegramFn
	notifyTelegramFn = func(string) {}
	return func() {
		worktreePathFn = origPath
		notifyTelegramFn = origNotify
	}
}

// TestHandleWorkerPRMerge_DirtyWorktree verifies that handleWorkerPRMerge returns
// AdvanceStatusRejected when the worktree has uncommitted changes.
func TestHandleWorkerPRMerge_DirtyWorktree(t *testing.T) {
	worktreeDir := filepath.Join(t.TempDir(), "abcd1234-myproj")
	setupDirtyRepo(t, worktreeDir)
	defer stubWorktreePathAndNotify(t, worktreeDir)()

	task := &taskwarrior.Task{
		UUID:        "abcd1234-0000-0000-0000-000000000000",
		Project:     "myproj",
		Description: "test task",
	}

	w := httptest.NewRecorder()
	done := handleWorkerPRMerge(w, task)

	if !done {
		t.Fatal("expected handleWorkerPRMerge to return true (response written)")
	}
	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != AdvanceStatusRejected {
		t.Errorf("expected status %q, got %q", AdvanceStatusRejected, resp.Status)
	}
	if !strings.Contains(resp.Message, "uncommitted changes") {
		t.Errorf("expected message about uncommitted changes, got %q", resp.Message)
	}
}

// TestHandleWorkerPRMerge_CleanWorktree verifies that handleWorkerPRMerge proceeds
// to the merge attempt when the worktree is clean.
func TestHandleWorkerPRMerge_CleanWorktree(t *testing.T) {
	worktreeDir := filepath.Join(t.TempDir(), "abcd1234-myproj")
	setupDirtyRepo(t, worktreeDir)
	// Stage and commit the modification so the worktree is clean.
	runGit(t, "git", "-C", worktreeDir, "add", ".")
	runGit(t, "git", "-C", worktreeDir, "commit", "-m", "clean")
	defer stubWorktreePathAndNotify(t, worktreeDir)()

	task := &taskwarrior.Task{
		UUID:        "abcd1234-0000-0000-0000-000000000000",
		Project:     "myproj",
		Description: "test task",
	}

	w := httptest.NewRecorder()
	handleWorkerPRMerge(w, task)

	// Guard should not block — merge proceeds and fails with an error (no real project config).
	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status == AdvanceStatusRejected {
		t.Errorf("clean worktree should not be blocked, got status %q: %s", resp.Status, resp.Message)
	}
	if resp.Status != AdvanceStatusError {
		t.Errorf("expected merge to be attempted (AdvanceStatusError from missing config), got %q", resp.Status)
	}
}

// TestHandleWorkerPRMerge_MissingWorktree verifies that handleWorkerPRMerge proceeds
// when the worktree directory does not exist (already cleaned up).
func TestHandleWorkerPRMerge_MissingWorktree(t *testing.T) {
	// Point to a non-existent dir — guard skips, merge is attempted.
	missingDir := filepath.Join(t.TempDir(), "abcd1234-myproj")
	defer stubWorktreePathAndNotify(t, missingDir)()

	task := &taskwarrior.Task{
		UUID:        "abcd1234-0000-0000-0000-000000000000",
		Project:     "myproj",
		Description: "test task",
	}

	w := httptest.NewRecorder()
	handleWorkerPRMerge(w, task)

	// Guard is skipped for absent dir — merge proceeds and fails with an error (no real project config).
	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status == AdvanceStatusRejected {
		t.Errorf("expected guard skipped for missing worktree, but got status %q", resp.Status)
	}
	if resp.Status != AdvanceStatusError {
		t.Errorf("expected merge to be attempted (AdvanceStatusError from missing config), got %q", resp.Status)
	}
}

// setupDirtyRepo initialises a git repo in dir, makes an initial commit,
// then modifies a tracked file without staging — leaving the worktree dirty.
func setupDirtyRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, "git", "-C", dir, "init")
	runGit(t, "git", "-C", dir, "config", "user.email", "test@test.com")
	runGit(t, "git", "-C", dir, "config", "user.name", "Test")
	// Create and commit a file.
	tracked := filepath.Join(dir, "tracked.txt")
	if err := os.WriteFile(tracked, []byte("initial"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, "git", "-C", dir, "add", ".")
	runGit(t, "git", "-C", dir, "commit", "-m", "initial")
	// Modify without staging — makes the worktree dirty.
	if err := os.WriteFile(tracked, []byte("modified"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// runGit runs a git command for test setup, fatal on failure.
func runGit(t *testing.T, args ...string) {
	t.Helper()
	//nolint:gosec // test helper only
	cmd := exec.Command(args[0], args[1:]...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git command %v failed: %v\n%s", args, err, out)
	}
}

// TestResolveReviewerSession verifies the resolveReviewerSession helper.
func TestResolveReviewerSession(t *testing.T) {
	agentRoles := map[string]string{
		testAgentInke: "designer",
		"athena":      "researcher",
	}
	const team = "default"
	const callerSession = "ttal-default-yuki"

	t.Run("agent tag found returns owner session", func(t *testing.T) {
		tags := []string{"feature", testAgentInke, "plan"}
		got := resolveReviewerSession(tags, agentRoles, team, callerSession)
		want := config.AgentSessionName(team, testAgentInke)
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("no agent tag falls back to caller session", func(t *testing.T) {
		tags := []string{"feature", "plan"}
		got := resolveReviewerSession(tags, agentRoles, team, callerSession)
		if got != callerSession {
			t.Errorf("expected caller session %q, got %q", callerSession, got)
		}
	})

	t.Run("tag not in agentRoles falls back to caller session", func(t *testing.T) {
		tags := []string{"feature", "someothertag", "plan"}
		got := resolveReviewerSession(tags, agentRoles, team, callerSession)
		if got != callerSession {
			t.Errorf("expected caller session %q, got %q", callerSession, got)
		}
	})

	t.Run("empty agentRoles falls back to caller session", func(t *testing.T) {
		tags := []string{"feature", testAgentInke, "plan"}
		got := resolveReviewerSession(tags, map[string]string{}, team, callerSession)
		if got != callerSession {
			t.Errorf("expected caller session %q, got %q", callerSession, got)
		}
	})

	t.Run("empty team falls back to caller session", func(t *testing.T) {
		tags := []string{"feature", testAgentInke, "plan"}
		got := resolveReviewerSession(tags, agentRoles, "", callerSession)
		if got != callerSession {
			t.Errorf("expected caller session %q, got %q", callerSession, got)
		}
	})
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

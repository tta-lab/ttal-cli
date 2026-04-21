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

// TestHandleWorkerPRMerge_CIPending verifies that handleWorkerPRMerge returns
// AdvanceStatusNeedsLGTM (not an error) when mergeWorkerPRFn returns ErrCIPending.
func TestHandleWorkerPRMerge_CIPending(t *testing.T) {
	missingDir := filepath.Join(t.TempDir(), "abcd1234-myproj")
	var notified []string

	origPath := worktreePathFn
	origNotify := notifyTelegramFn
	origMerge := mergeWorkerPRFn
	worktreePathFn = func(_, _ string) (string, error) { return missingDir, nil }
	notifyTelegramFn = func(msg string) { notified = append(notified, msg) }
	mergeWorkerPRFn = func(_ *taskwarrior.Task) error { return ErrCIPending }
	defer func() {
		worktreePathFn = origPath
		notifyTelegramFn = origNotify
		mergeWorkerPRFn = origMerge
	}()

	task := &taskwarrior.Task{
		UUID:        "abcd1234-0000-0000-0000-000000000000",
		Project:     "myproj",
		Description: "test task",
		PRID:        "myproj/myrepo#42",
	}

	w := httptest.NewRecorder()
	done := handleWorkerPRMerge(w, task)

	if !done {
		t.Fatal("expected handleWorkerPRMerge to return true (response written)")
	}

	var resp AdvanceResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != AdvanceStatusNeedsLGTM {
		t.Errorf("expected AdvanceStatusNeedsLGTM for CI-pending, got %q: %s", resp.Status, resp.Message)
	}
	if !strings.Contains(resp.Message, "CI checks still running") {
		t.Errorf("expected CI-pending message, got %q", resp.Message)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected HTTP 200 for CI-pending (not an error), got %d", w.Code)
	}

	// Should have sent a Telegram notification about the pending CI merge.
	ciPendingNotified := false
	for _, msg := range notified {
		if strings.Contains(msg, "CI checks still running") || strings.Contains(msg, "merge blocked") {
			ciPendingNotified = true
			break
		}
	}
	if !ciPendingNotified {
		t.Errorf("expected CIPendingMerge Telegram notification, got: %v", notified)
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
	const callerSession = "ttal-default-yuki"

	t.Run("owner set returns owner session", func(t *testing.T) {
		task := &taskwarrior.Task{UUID: "t1", Owner: "inke"}
		got := resolveReviewerSession(task, callerSession)
		want := config.AgentSessionName("inke")
		if got != want {
			t.Errorf("expected %q, got %q", want, got)
		}
	})

	t.Run("owner empty falls back to caller session", func(t *testing.T) {
		task := &taskwarrior.Task{UUID: "t2"} // no Owner
		got := resolveReviewerSession(task, callerSession)
		if got != callerSession {
			t.Errorf("expected caller session %q, got %q", callerSession, got)
		}
	})
}

// TestCheckOwnershipGuard verifies that the ownership guard allows or rejects correctly.
func TestCheckOwnershipGuard(t *testing.T) {
	agentRoles := map[string]string{
		testAgentInke: "designer",
		"yuki":        "manager",
		"athena":      "researcher",
	}

	newTask := func(owner string) *taskwarrior.Task {
		return &taskwarrior.Task{
			UUID:  "abcd1234-0000-0000-0000-000000000000",
			Owner: owner,
		}
	}

	t.Run("owner calls go on own task — allowed", func(t *testing.T) {
		task := newTask("inke")
		w := httptest.NewRecorder()
		rejected := checkOwnershipGuard(w, task, "inke", agentRoles)
		if rejected {
			t.Error("owner should be allowed to advance their own task")
		}
	})

	t.Run("non-owner manager calls go on owned task — rejected", func(t *testing.T) {
		task := newTask("inke") // owned by inke
		w := httptest.NewRecorder()
		rejected := checkOwnershipGuard(w, task, "yuki", agentRoles)
		if !rejected {
			t.Error("non-owner manager should be rejected")
		}
		var resp AdvanceResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if resp.Status != AdvanceStatusRejected {
			t.Errorf("expected status %q, got %q", AdvanceStatusRejected, resp.Status)
		}
		if !strings.Contains(resp.Message, "inke") {
			t.Errorf("message should name the owner %q: %s", "inke", resp.Message)
		}
	})

	t.Run("unowned task — allowed", func(t *testing.T) {
		task := newTask("") // no Owner
		w := httptest.NewRecorder()
		rejected := checkOwnershipGuard(w, task, "yuki", agentRoles)
		if rejected {
			t.Error("unowned task should always be allowed (routing phase)")
		}
	})

	t.Run("empty callerAgent (worker) — allowed", func(t *testing.T) {
		task := newTask("inke")
		w := httptest.NewRecorder()
		rejected := checkOwnershipGuard(w, task, "", agentRoles)
		if rejected {
			t.Error("empty callerAgent should be allowed (worker)")
		}
	})

	t.Run("caller not in agentRoles (worker session name) — allowed", func(t *testing.T) {
		task := newTask("inke")
		w := httptest.NewRecorder()
		rejected := checkOwnershipGuard(w, task, "3860d481-ttal", agentRoles)
		if rejected {
			t.Error("unknown agent name (worker session) should be allowed")
		}
	})
}

// TestAdvance_SecondManagerRoute_OwnerUnchanged verifies that setOwnerFn is NOT called
// when routing a task that already has an owner set (write-once guard).
func TestAdvance_SecondManagerRoute_OwnerUnchanged(t *testing.T) {
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		t.Errorf("setOwnerFn should not be called on second manager route, got uuid=%s owner=%s", uuid, owner)
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })
}

// TestEnsureWorkerStageOwner_WriteOnceGuard verifies that ensureWorkerStageOwner
// does NOT call setOwnerFn when the task already has an owner (write-once guard).
func TestEnsureWorkerStageOwner_WriteOnceGuard(t *testing.T) {
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		t.Errorf("setOwnerFn should not be called at worker stage, got uuid=%s owner=%s", uuid, owner)
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })
}

// TestAdvance_WorkerStageFromUnowned_SetsOwner verifies that setOwnerFn IS called
// with the caller when an unowned task routes to a worker stage (e.g. hotfix).
func TestEnsureWorkerStageOwner_SetsOwnerForUnownedTask(t *testing.T) {
	origCount := countActiveTasksByOwnerFn
	countActiveTasksByOwnerFn = func(string) (int, error) { return 0, nil }
	t.Cleanup(func() { countActiveTasksByOwnerFn = origCount })

	var capturedOwner string
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		capturedOwner = owner
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })

	task := &taskwarrior.Task{UUID: "aaaa0001", Owner: ""}
	err := ensureWorkerStageOwner(task, "cael", "Implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOwner != "cael" {
		t.Errorf("capturedOwner = %q, want %q", capturedOwner, "cael")
	}
}

func TestEnsureWorkerStageOwner_SkipsOwnedTask(t *testing.T) {
	var called bool
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })

	task := &taskwarrior.Task{UUID: "aaaa0001", Owner: "astra"}
	err := ensureWorkerStageOwner(task, "cael", "Implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("setOwnerFn should not be called for already-owned task")
	}
}

// TestResolveStageAgent_OwnerEmpty_PicksIdleAgent verifies that with no owner set,
// findIdleAgent is called to pick an idle agent.
func TestResolveStageAgent_OwnerEmpty_PicksIdleAgent(t *testing.T) {
	dir := t.TempDir()
	agentMD := "---\nrole: designer\n---\n# Inke\n"
	os.MkdirAll(filepath.Join(dir, testAgentInke), 0o755) //nolint:errcheck
	if err := os.WriteFile(filepath.Join(dir, testAgentInke, "AGENTS.md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}

	origCount := countActiveTasksByOwnerFn
	countActiveTasksByOwnerFn = func(string) (int, error) { return 0, nil }
	t.Cleanup(func() { countActiveTasksByOwnerFn = origCount })

	agentRoles := map[string]string{testAgentInke: "designer"}
	agent, err := resolveStageAgent(dir, "", "designer", agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Name != testAgentInke {
		t.Errorf("expected agent %q, got %q", testAgentInke, agent.Name)
	}
}

// TestResolveStageAgent_OwnerSet_RoleMatches verifies that with a set owner whose role
// matches, the owner agent is returned WITHOUT consulting the busy check.
func TestResolveStageAgent_OwnerSet_RoleMatches(t *testing.T) {
	dir := t.TempDir()
	agentMD := "---\nrole: designer\n---\n# Inke\n"
	os.MkdirAll(filepath.Join(dir, testAgentInke), 0o755) //nolint:errcheck
	if err := os.WriteFile(filepath.Join(dir, testAgentInke, "AGENTS.md"), []byte(agentMD), 0o644); err != nil {
		t.Fatal(err)
	}

	// If countActiveTasksByOwnerFn is called, fail the test — owner branch must skip busy check.
	countActiveTasksByOwnerFn = func(string) (int, error) {
		t.Fatal("countActiveTasksByOwnerFn should not be called for set owner")
		return 0, nil
	}
	t.Cleanup(func() { countActiveTasksByOwnerFn = func(string) (int, error) { return 0, nil } })

	agentRoles := map[string]string{testAgentInke: "designer"}
	agent, err := resolveStageAgent(dir, testAgentInke, "designer", agentRoles)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agent.Name != testAgentInke {
		t.Errorf("expected agent %q, got %q", testAgentInke, agent.Name)
	}
}

// TestResolveStageAgent_OwnerSet_RoleMismatch_Errors verifies that a set owner with a
// mismatched role returns a hard error, not a fallback.
func TestResolveStageAgent_OwnerSet_RoleMismatch_Errors(t *testing.T) {
	agentRoles := map[string]string{testAgentInke: "designer"}
	_, err := resolveStageAgent("/tmp", testAgentInke, "coder", agentRoles)
	if err == nil {
		t.Fatal("expected error for role mismatch, got nil")
	}
}

// TestResolveStageAgent_OwnerNotRecognized_Errors verifies that an unknown owner returns
// a hard error.
func TestResolveStageAgent_OwnerNotRecognized_Errors(t *testing.T) {
	agentRoles := map[string]string{testAgentInke: "designer"}
	_, err := resolveStageAgent("/tmp", "unknown-agent", "designer", agentRoles)
	if err == nil {
		t.Fatal("expected error for unrecognized owner, got nil")
	}
}

// TestEnsureWorkerStageOwner_CallerBusy_Rejects verifies that a busy caller is rejected
// and setOwnerFn is NOT called.
func TestEnsureWorkerStageOwner_CallerBusy_Rejects(t *testing.T) {
	origCount := countActiveTasksByOwnerFn
	countActiveTasksByOwnerFn = func(owner string) (int, error) {
		if owner == "yuki" {
			return 1, nil
		}
		return 0, nil
	}
	t.Cleanup(func() { countActiveTasksByOwnerFn = origCount })

	var setCalled bool
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		setCalled = true
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })

	task := &taskwarrior.Task{UUID: "aaaa0001", Owner: ""}
	err := ensureWorkerStageOwner(task, "yuki", "Implement")
	if err == nil {
		t.Fatal("expected error for busy caller, got nil")
	}
	if setCalled {
		t.Error("setOwnerFn should not be called for busy caller")
	}
}

// TestEnsureWorkerStageOwner_CallerIdle_SetsOwner verifies that an idle caller
// successfully acquires ownership.
func TestEnsureWorkerStageOwner_CallerIdle_SetsOwner(t *testing.T) {
	origCount := countActiveTasksByOwnerFn
	countActiveTasksByOwnerFn = func(string) (int, error) { return 0, nil }
	t.Cleanup(func() { countActiveTasksByOwnerFn = origCount })

	var capturedOwner string
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		capturedOwner = owner
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })

	task := &taskwarrior.Task{UUID: "aaaa0001", Owner: ""}
	err := ensureWorkerStageOwner(task, "yuki", "Implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedOwner != "yuki" {
		t.Errorf("capturedOwner = %q, want %q", capturedOwner, "yuki")
	}
}

// TestEnsureWorkerStageOwner_EmptyCaller_Skips verifies that an empty callerAgent
// skips ownership write without error.
func TestEnsureWorkerStageOwner_EmptyCaller_Skips(t *testing.T) {
	var called bool
	orig := setOwnerFn
	setOwnerFn = func(uuid, owner string) error {
		called = true
		return nil
	}
	t.Cleanup(func() { setOwnerFn = orig })

	task := &taskwarrior.Task{UUID: "aaaa0001", Owner: ""}
	err := ensureWorkerStageOwner(task, "", "Implement")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("setOwnerFn should not be called for empty callerAgent")
	}
}

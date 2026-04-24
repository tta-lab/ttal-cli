package daemon

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

func TestIsValidHexPrefix(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"8 hex chars is valid", "abc12345", true},
		{"more than 8 hex chars is valid", "abc12345abcdef", true},
		{"7 hex chars is invalid", "abc1234", false},
		{"non-hex chars is invalid", "zzzzzzzz", false},
		{"empty string is invalid", "", false},
		{"mixed case hex is valid", "ABCDEF12", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidHexPrefix(tt.input)
			if got != tt.want {
				t.Errorf("isValidHexPrefix(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsBareWorkerHex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"bare hex is valid", "abc12345", true},
		{"bare hex with colon is invalid", "abc12345:coder", false},
		{"short bare hex is invalid", "abc123", false},
		{"non-hex bare is invalid", "zzzzzzzz", false},
		{"mixed case bare hex is valid", "ABCDEF12", true},
		{"empty string is invalid", "", false},
		{"agent name without hex is invalid", "kestrel", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBareWorkerHex(tt.input)
			if got != tt.want {
				t.Errorf("isBareWorkerHex(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseWorkerAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantJob  string
		wantName string
		wantOK   bool
	}{
		{
			name:     "valid job_id:agent_name",
			input:    "abc12345:coder",
			wantJob:  "abc12345",
			wantName: "coder",
			wantOK:   true,
		},
		{
			name:     "valid mixed case",
			input:    "AABBCCDD:reviewer",
			wantJob:  "AABBCCDD",
			wantName: "reviewer",
			wantOK:   true,
		},
		{
			name:     "exact 8 chars",
			input:    "aabbccdd:coder",
			wantJob:  "aabbccdd",
			wantName: "coder",
			wantOK:   true,
		},
		{
			name:     "multi-colon (agent_name contains colon)",
			input:    "abc12345:team:coder",
			wantJob:  "abc12345",
			wantName: "team:coder",
			wantOK:   true,
		},
		{
			name:   "bare hex is invalid",
			input:  "abc12345",
			wantOK: false,
		},
		{
			name:   "no colon",
			input:  "kestrel",
			wantOK: false,
		},
		{
			name:   "short hex",
			input:  "abc:coder",
			wantOK: false,
		},
		{
			name:   "non-hex prefix",
			input:  "zzzzzzzz:coder",
			wantOK: false,
		},
		{
			name:   "empty agent name",
			input:  "abc12345:",
			wantOK: false,
		},
		{
			name:   "empty string",
			input:  "",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobID, agentName, ok := parseWorkerAddress(tt.input)
			if ok != tt.wantOK {
				t.Errorf("parseWorkerAddress(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
				return
			}
			if tt.wantOK {
				if jobID != tt.wantJob {
					t.Errorf("parseWorkerAddress(%q) jobID = %q, want %q", tt.input, jobID, tt.wantJob)
				}
				if agentName != tt.wantName {
					t.Errorf("parseWorkerAddress(%q) agentName = %q, want %q", tt.input, agentName, tt.wantName)
				}
			}
		})
	}
}

// TestHandleAgentToAgentUnknownSender verifies the negative-path behaviour for
// the From field in handleAgentToAgent after the worker-hex-ID fix.
//
// Success-path verification (a real worker session sending an alert) requires a
// live tmux server and cannot run in CI. These tests cover the regression:
// before the fix, any unknown From returned "unknown agent: X". After the fix,
// only truly unresolvable senders error — and the error message says
// "unknown agent or worker".
func TestHandleAgentToAgentUnknownSender(t *testing.T) {
	mcfg := &config.Config{}

	tests := []struct {
		name    string
		from    string
		wantErr string
	}{
		{
			// Non-hex string: fails parseWorkerAddress's character check.
			name:    "non-hex from is rejected",
			from:    "not-a-worker",
			wantErr: "unknown agent or worker",
		},
		{
			// Short hex (< 8 chars): fails parseWorkerAddress's length check.
			name:    "short hex from is rejected",
			from:    "abc123",
			wantErr: "unknown agent or worker",
		},
		{
			// Non-hex 8-char string: fails parseWorkerAddress's character loop.
			name:    "non-hex 8-char from is rejected",
			from:    "gggggggg",
			wantErr: "unknown agent or worker",
		},
		{
			// Compound format but no matching tmux session — resolveWorker errors.
			name:    "compound with no tmux server is rejected",
			from:    "aabbccdd:coder",
			wantErr: "unknown agent or worker",
		},
		{
			// Bare hex (old format): resolveWorker would succeed but we now reject it.
			name:    "bare hex from is rejected",
			from:    "aabbccdd",
			wantErr: "bare worker UUID not supported",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := SendRequest{
				From:    tt.from,
				To:      "kestrel",
				Message: "test",
			}
			err := handleAgentToAgent(mcfg, nil, nil, nil, req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestBareHexError verifies bareHexError does not panic on empty string.
func TestBareHexError(t *testing.T) {
	// Must not panic
	err := bareHexError("")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "abc12345:coder") {
		t.Errorf("error = %q, want example abc12345:coder", err.Error())
	}

	// With 8+ chars should use the provided prefix
	err = bareHexError("aabbccdd")
	if !strings.Contains(err.Error(), "aabbccdd:coder") {
		t.Errorf("error = %q, want prefix aabbccdd:coder", err.Error())
	}
}

// TestHandleToRejectsBareHex verifies that handleTo rejects bare hex UUIDs
// with a helpful error message.
func TestHandleToRejectsBareHex(t *testing.T) {
	mcfg := &config.Config{}

	req := SendRequest{
		To:      "aabbccdd",
		Message: "test",
	}
	err := handleTo(mcfg, nil, nil, nil, req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bare worker UUID not supported") {
		t.Errorf("error = %q, want substring %q", err.Error(), "bare worker UUID not supported")
	}
	if !strings.Contains(err.Error(), "job_id:agent_name") {
		t.Errorf("error = %q, want substring %q", err.Error(), "job_id:agent_name")
	}
}

// TestResolveManagerWindow tests the resolveManagerWindow function with injected mocks.
const testJobIDA = "e9d4b7c1"

// expectedSessionAstra is the tmux session name for the "astra" agent with default team.
const expectedSessionAstra = "ttal-default-astra"

// pipelineConfigForTest returns a pipeline.Config with a single stage that is NOT a worker stage.
func pipelineConfigForTest(workerStage bool) *pipeline.Config {
	stages := []pipeline.Stage{
		{Name: "Plan", Assignee: "astra", Reviewer: "plan-review-lead"},
	}
	if workerStage {
		stages = append(stages, pipeline.Stage{Name: "Implement", Assignee: "coder", Worker: true})
	}
	return &pipeline.Config{
		Pipelines: map[string]pipeline.Pipeline{
			"default": {Tags: []string{"feature"}, Stages: stages},
		},
	}
}

//nolint:gocyclo
func TestResolveManagerWindow(t *testing.T) {
	mcfg := &config.Config{}

	origExport := exportTaskByHexIDFn
	origWindowExists := windowExistsFn
	origPipelineLoad := pipelineLoadFn
	t.Cleanup(func() {
		exportTaskByHexIDFn = origExport
		windowExistsFn = origWindowExists
		pipelineLoadFn = origPipelineLoad
	})

	taskWithOwner := &taskwarrior.Task{
		UUID:        "e9d4b7c1aabbccddeeff001122334455",
		Description: "test task",
		Tags:        []string{"feature", "stage:plan"},
		Status:      "pending",
		Owner:       "astra",
	}
	taskWithoutOwner := &taskwarrior.Task{
		UUID:        "e9d4b7c1aabbccddeeff001122334466",
		Description: "test task no owner",
		Tags:        []string{"feature", "stage:plan"},
		Status:      "pending",
		Owner:       "",
	}
	taskAtWorkerStage := &taskwarrior.Task{
		UUID:        "e9d4b7c1aabbccddeeff001122334477",
		Description: "test task at worker stage",
		Tags:        []string{"feature", "implement"},
		Status:      "pending",
		Owner:       "astra",
	}

	t.Run("returns correct session when task has owner and current stage is manager", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithOwner, nil
			}
			return nil, errors.New("not found")
		}
		pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
			return pipelineConfigForTest(false), nil
		}
		windowExistsFn = func(session, window string) bool {
			return session == expectedSessionAstra && window == "coder"
		}

		session, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session != expectedSessionAstra {
			t.Errorf("session = %q, want %q", session, expectedSessionAstra)
		}
	})

	t.Run("returns error when no owner on task", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithoutOwner, nil
			}
			return nil, errors.New("not found")
		}
		pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
			return pipelineConfigForTest(false), nil
		}

		_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no owner on task") {
			t.Errorf("error = %q, want substring %q", err.Error(), "no owner on task")
		}
	})

	t.Run("returns error when current stage is worker", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskAtWorkerStage, nil
			}
			return nil, errors.New("not found")
		}
		pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
			return pipelineConfigForTest(true), nil
		}

		_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "worker stage") {
			t.Errorf("error = %q, want substring %q", err.Error(), "worker stage")
		}
	})

	t.Run("returns error when window not found in session", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithOwner, nil
			}
			return nil, errors.New("not found")
		}
		pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
			return pipelineConfigForTest(false), nil
		}
		windowExistsFn = func(session, window string) bool {
			return false // window does not exist
		}

		_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "window") || !strings.Contains(err.Error(), "not found") {
			t.Errorf("error = %q, want substring containing 'window' and 'not found'", err.Error())
		}
	})

	t.Run("returns error when no pipeline matches task tags", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithOwner, nil
			}
			return nil, errors.New("not found")
		}
		// A pipeline with a different tag — task tags ("feature", "stage:plan") won't match.
		pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
			return &pipeline.Config{
				Pipelines: map[string]pipeline.Pipeline{
					"other": {Tags: []string{"chore"}, Stages: []pipeline.Stage{
						{Name: "Plan", Assignee: "astra"},
					}},
				},
			}, nil
		}

		_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no pipeline matches") {
			t.Errorf("error = %q, want substring %q", err.Error(), "no pipeline matches")
		}
	})
}

// TestResolveManagerWindowTaskLookupError verifies that resolveManagerWindow propagates
// task lookup errors from the injected exportTaskByHexIDFn.
func TestResolveManagerWindowTaskLookupError(t *testing.T) {
	mcfg := &config.Config{}

	origExport := exportTaskByHexIDFn
	origPipelineLoad := pipelineLoadFn
	t.Cleanup(func() {
		exportTaskByHexIDFn = origExport
		pipelineLoadFn = origPipelineLoad
	})

	exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
		return nil, errors.New("task not found")
	}
	pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
		return pipelineConfigForTest(false), nil
	}

	_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task lookup") {
		t.Errorf("error = %q, want substring %q", err.Error(), "task lookup")
	}
}

// TestResolveManagerWindowWithTeam verifies AgentSessionName formatting with a non-empty team.
func TestResolveManagerWindowWithTeam(t *testing.T) {
	origExport := exportTaskByHexIDFn
	origWindowExists := windowExistsFn
	origPipelineLoad := pipelineLoadFn
	t.Cleanup(func() {
		exportTaskByHexIDFn = origExport
		windowExistsFn = origWindowExists
		pipelineLoadFn = origPipelineLoad
	})

	cfg := &config.Config{}

	taskWithOwner := &taskwarrior.Task{
		UUID:        testJobIDA + "aabbccddeeff",
		Description: "test",
		Tags:        []string{"feature", "stage:plan"},
		Owner:       "astra",
		Status:      "pending",
	}

	exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
		if hexID == testJobIDA {
			return taskWithOwner, nil
		}
		return nil, errors.New("not found")
	}
	pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
		return pipelineConfigForTest(false), nil
	}
	windowExistsFn = func(session, window string) bool {
		// session is always ttal-default-<owner> after single-team collapse
		return session == expectedSessionAstra && window == "subagent"
	}

	session, err := resolveManagerWindow(testJobIDA, "subagent", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session != expectedSessionAstra {
		t.Errorf("session = %q, want %q", session, expectedSessionAstra)
	}
}

// TestResolveManagerWindow_EmptyTeamPath_Regression is deleted — empty team name is
// impossible with single-team hardcoded sessions.

// TestResolveManagerWindow_RealLoadAll exercises AgentSessionName through the real
// config.Load() path — NOT hand-stuffed mcfg. This is the regression gap PR #516
// fell into: hand-populated mcfg bypassed LoadAll() and the missing resolvedTeamName
// was invisible to tests.
func TestResolveManagerWindow_RealLoadAll(t *testing.T) {
	// Load() reads ~/.config/ttal/config.toml — skip if absent (CI, bare containers).
	realCfgPath := filepath.Join(config.DefaultConfigDir(), "config.toml")
	if _, err := os.Stat(realCfgPath); os.IsNotExist(err) {
		t.Skip("no real config at ~/.config/ttal/config.toml — skipping integration test")
	}
	mcfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	origExport := exportTaskByHexIDFn
	origWindowExists := windowExistsFn
	origPipelineLoad := pipelineLoadFn
	t.Cleanup(func() {
		exportTaskByHexIDFn = origExport
		windowExistsFn = origWindowExists
		pipelineLoadFn = origPipelineLoad
	})

	taskOwner := &taskwarrior.Task{
		UUID:        "e9d4b7c1",
		Description: "test",
		Tags:        []string{"feature"},
		Owner:       "kestrel",
		Status:      "pending",
	}
	exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
		if hexID == "e9d4b7c1" {
			return taskOwner, nil
		}
		return nil, errors.New("not found")
	}
	pipelineLoadFn = func(dir string) (*pipeline.Config, error) {
		return pipelineConfigForTest(false), nil
	}
	windowExistsFn = func(session, window string) bool {
		// Stub — we only check the return value, not this side-effect
		return true
	}

	session, err := resolveManagerWindowImpl("e9d4b7c1", "design", mcfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// This is the critical assertion: AgentSessionName must produce "ttal-default-kestrel",
	// NOT "ttal--kestrel" (double dash from empty team) or "ttal-<team>-kestrel".
	if session != "ttal-default-kestrel" {
		t.Errorf("session = %q, want %q", session, "ttal-default-kestrel")
	}
}

// TestDispatchToWorkerOrManager tests the dispatchToWorkerOrManager function.
//
//nolint:gocyclo
func TestDispatchToWorkerOrManager(t *testing.T) {
	mcfg := &config.Config{}

	origResolveWorker := resolveWorker
	origResolveManagerWindow := resolveManagerWindow
	origDispatchImpl := dispatchToWorkerImpl
	t.Cleanup(func() {
		resolveWorker = origResolveWorker
		resolveManagerWindow = origResolveManagerWindow
		dispatchToWorkerImpl = origDispatchImpl
	})

	// Track which dispatch path was called.
	var dispatchCalls []string
	dispatchToWorkerImpl = func(
		msgSvc *message.Service, session, windowName string, params message.CreateParams, text string,
	) error {
		dispatchCalls = append(dispatchCalls, session+":"+windowName)
		return nil
	}

	t.Run("worker found returns worker dispatch", func(t *testing.T) {
		dispatchCalls = nil
		resolveWorker = func(idPrefix string) (string, error) {
			return "w-e9d4b7c1-coder", nil
		}
		resolveManagerWindow = func(jobID, windowName string, m *config.Config) (string, error) {
			t.Fatal("resolveManagerWindow should not be called when worker is found")
			return "", nil
		}

		session, dispatched, err := dispatchToWorkerOrManager(
			mcfg, "e9d4b7c1", "coder", nil, "yuki", "ttal", "e9d4b7c1:coder", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !dispatched {
			t.Error("expected dispatched=true")
		}
		if session != "w-e9d4b7c1-coder" {
			t.Errorf("session = %q, want %q", session, "w-e9d4b7c1-coder")
		}
		if len(dispatchCalls) != 1 || dispatchCalls[0] != "w-e9d4b7c1-coder:coder" {
			t.Errorf("dispatch calls = %v, want [w-e9d4b7c1-coder:coder]", dispatchCalls)
		}
	})

	t.Run("worker gone plus manager found returns manager fallback dispatch", func(t *testing.T) {
		dispatchCalls = nil
		resolveWorker = func(idPrefix string) (string, error) {
			return "", errors.New("no worker session")
		}
		resolveManagerWindow = func(jobID, windowName string, m *config.Config) (string, error) {
			if jobID == "e9d4b7c1" && windowName == "coder" {
				return "ttal-default-yuki", nil
			}
			return "", errors.New("manager not found")
		}

		session, dispatched, err := dispatchToWorkerOrManager(
			mcfg, "e9d4b7c1", "coder", nil, "worker", "ttal", "e9d4b7c1:coder", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !dispatched {
			t.Error("expected dispatched=true")
		}
		if session != "ttal-default-yuki" {
			t.Errorf("session = %q, want %q", session, "ttal-default-yuki")
		}
		if len(dispatchCalls) != 1 || dispatchCalls[0] != "ttal-default-yuki:coder" {
			t.Errorf("dispatch calls = %v, want [ttal-default-yuki:coder]", dispatchCalls)
		}
	})

	t.Run("both fail returns error", func(t *testing.T) {
		dispatchCalls = nil
		resolveWorker = func(idPrefix string) (string, error) {
			return "", errors.New("no worker session")
		}
		resolveManagerWindow = func(jobID, windowName string, m *config.Config) (string, error) {
			return "", errors.New("manager window not found")
		}

		session, dispatched, err := dispatchToWorkerOrManager(
			mcfg, "e9d4b7c1", "coder", nil, "yuki", "ttal", "e9d4b7c1:coder", nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if dispatched {
			t.Error("expected dispatched=false")
		}
		if session != "" {
			t.Errorf("session = %q, want empty", session)
		}
		if len(dispatchCalls) != 0 {
			t.Errorf("dispatch calls = %v, want []", dispatchCalls)
		}
	})
}

// TestResolveAddressee_Unknown verifies that unknown names produce an error listing known agents and humans.
func TestResolveAddressee_Unknown(t *testing.T) {
	// Test that unknown names produce the expected error format
	cfg := &config.Config{}
	addr, err := resolveAddressee(cfg, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown name")
	}
	if addr != nil {
		t.Errorf("expected nil addressee, got %+v", addr)
	}
	if !strings.Contains(err.Error(), "unknown addressee: unknown") {
		t.Errorf("error = %q, want substring %q", err.Error(), "unknown addressee: unknown")
	}
}

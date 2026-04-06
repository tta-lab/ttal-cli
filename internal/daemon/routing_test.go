package daemon

import (
	"errors"
	"strings"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/config"
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
	mcfg := &config.DaemonConfig{}

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
	mcfg := &config.DaemonConfig{}

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

func TestResolveManagerWindow(t *testing.T) {
	mcfg := &config.DaemonConfig{Global: &config.Config{}}

	origExport := exportTaskByHexIDFn
	origWindowExists := windowExistsFn
	origBuildAgentRoles := buildAgentRolesFn
	t.Cleanup(func() {
		exportTaskByHexIDFn = origExport
		windowExistsFn = origWindowExists
		buildAgentRolesFn = origBuildAgentRoles
	})

	taskWithOwner := &taskwarrior.Task{
		UUID:        "e9d4b7c1aabbccddeeff001122334455",
		Description: "test task",
		Tags:        []string{"feature", "yuki"},
		Status:      "pending",
	}
	taskWithoutOwner := &taskwarrior.Task{
		UUID:        "e9d4b7c1aabbccddeeff001122334466",
		Description: "test task no owner",
		Tags:        []string{"feature"},
		Status:      "pending",
	}

	// Inject a buildAgentRolesFn that returns a known role map.
	injectRoles := func(roles map[string]string) {
		buildAgentRolesFn = func(teamPath string) map[string]string { return roles }
	}
	injectRoles(map[string]string{"yuki": "manager", "kestrel": "fixer"})

	t.Run("returns correct session when task has agent tag and window exists", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithOwner, nil
			}
			return nil, errors.New("not found")
		}
		// TeamName() returns "" for an empty Config, so session is "ttal--yuki".
		windowExistsFn = func(session, window string) bool {
			return session == "ttal--yuki" && window == "coder"
		}

		session, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if session != "ttal--yuki" {
			t.Errorf("session = %q, want %q", session, "ttal--yuki")
		}
	})

	t.Run("returns error when no agent tag on task", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithoutOwner, nil
			}
			return nil, errors.New("not found")
		}

		_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "no owner agent tag") {
			t.Errorf("error = %q, want substring %q", err.Error(), "no owner agent tag")
		}
	})

	t.Run("returns error when window not found in session", func(t *testing.T) {
		exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
			if hexID == testJobIDA {
				return taskWithOwner, nil
			}
			return nil, errors.New("not found")
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
}

// TestResolveManagerWindowTaskLookupError verifies that resolveManagerWindow propagates
// task lookup errors from the injected exportTaskByHexIDFn.
func TestResolveManagerWindowTaskLookupError(t *testing.T) {
	mcfg := &config.DaemonConfig{Global: &config.Config{}}

	origExport := exportTaskByHexIDFn
	t.Cleanup(func() { exportTaskByHexIDFn = origExport })

	exportTaskByHexIDFn = func(hexID, status string) (*taskwarrior.Task, error) {
		return nil, errors.New("task not found")
	}

	_, err := resolveManagerWindow(testJobIDA, "coder", mcfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "task lookup") {
		t.Errorf("error = %q, want substring %q", err.Error(), "task lookup")
	}
}

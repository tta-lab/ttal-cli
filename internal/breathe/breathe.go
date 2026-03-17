package breathe

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/watcher"
)

// SessionConfig holds the parameters for creating a synthetic CC session.
type SessionConfig struct {
	CWD       string // agent's working directory
	CCVersion string // CC version (e.g. "2.1.47") — may be empty
	GitBranch string // current git branch
	Handoff   string // the handoff prompt content
}

// fileHistorySnapshot is the first line of a CC session JSONL.
type fileHistorySnapshot struct {
	Type             string   `json:"type"`
	MessageID        string   `json:"messageId"`
	Snapshot         snapshot `json:"snapshot"`
	IsSnapshotUpdate bool     `json:"isSnapshotUpdate"`
}

type snapshot struct {
	MessageID          string            `json:"messageId"`
	TrackedFileBackups map[string]string `json:"trackedFileBackups"`
	Timestamp          string            `json:"timestamp"`
}

// userMessage is the second line — the handoff prompt as a user message.
type userMessage struct {
	ParentUUID     *string       `json:"parentUuid"`
	IsSidechain    bool          `json:"isSidechain"`
	UserType       string        `json:"userType"`
	CWD            string        `json:"cwd"`
	SessionID      string        `json:"sessionId"`
	Version        string        `json:"version"`
	GitBranch      string        `json:"gitBranch"`
	Type           string        `json:"type"`
	Message        messageBody   `json:"message"`
	UUID           string        `json:"uuid"`
	Timestamp      string        `json:"timestamp"`
	Todos          []interface{} `json:"todos"`
	PermissionMode string        `json:"permissionMode"`
}

type messageBody struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// WriteSyntheticSession creates a minimal JSONL session file that CC can resume.
// Returns the new session ID. The JSONL is written to dir/<session-id>.jsonl.
func WriteSyntheticSession(dir string, cfg SessionConfig) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create session dir: %w", err)
	}

	sessionID := uuid.New().String()
	userMsgID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339Nano)

	// Line 1: file-history-snapshot
	snap := fileHistorySnapshot{
		Type:      "file-history-snapshot",
		MessageID: userMsgID,
		Snapshot: snapshot{
			MessageID:          userMsgID,
			TrackedFileBackups: map[string]string{},
			Timestamp:          now,
		},
		IsSnapshotUpdate: false,
	}

	// Line 2: user message with handoff
	msg := userMessage{
		ParentUUID:  nil,
		IsSidechain: false,
		UserType:    "external",
		CWD:         cfg.CWD,
		SessionID:   sessionID,
		Version:     cfg.CCVersion,
		GitBranch:   cfg.GitBranch,
		Type:        "user",
		Message: messageBody{
			Role:    "user",
			Content: cfg.Handoff,
		},
		UUID:           userMsgID,
		Timestamp:      now,
		Todos:          []interface{}{},
		PermissionMode: "bypassPermissions",
	}

	snapJSON, err := json.Marshal(snap)
	if err != nil {
		return "", fmt.Errorf("marshal snapshot: %w", err)
	}
	msgJSON, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal message: %w", err)
	}

	content := string(snapJSON) + "\n" + string(msgJSON) + "\n"

	path := filepath.Join(dir, sessionID+".jsonl")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write session JSONL: %w", err)
	}

	return sessionID, nil
}

// CCProjectDir returns the CC project directory for a given CWD.
// Uses watcher.EncodePath which replaces both "/" and "." with "-".
// Result: ~/.claude/projects/<encoded-cwd>/
func CCProjectDir(cwd string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "projects", watcher.EncodePath(cwd))
}

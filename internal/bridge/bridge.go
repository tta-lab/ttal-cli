package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// logBridge appends a debug line to ~/.ttal/bridge.log.
func logBridge(format string, args ...any) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	logPath := filepath.Join(home, ".ttal", "bridge.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "%s %s\n", time.Now().Format("15:04:05"), msg)
}

// StopHookInput is the JSON schema CC sends to Stop hooks via stdin.
type StopHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	StopHookActive bool   `json:"stop_hook_active"`
}

const (
	// tailLines is the number of lines to read from the end of the transcript.
	tailLines = 50

	// retryAttempts is how many times to re-read the JSONL waiting for the
	// current turn's assistant text to be flushed.
	retryAttempts = 5

	// retryDelay is how long to wait between retry attempts.
	retryDelay = 200 * time.Millisecond
)

// Run executes the bridge logic: read stdin, resolve agent, extract last
// assistant text from transcript, and send to daemon via socket.
func Run() error {
	var input StopHookInput
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		return nil // malformed stdin — silent exit
	}

	// Loop prevention
	if input.StopHookActive {
		return nil
	}

	if input.Cwd == "" || input.TranscriptPath == "" {
		return nil
	}

	// Resolve cwd → agent name via database
	agentName, err := resolveAgent(input.Cwd)
	if err != nil {
		return nil // no matching agent — silent exit
	}

	// Extract all assistant texts from the current turn with retry — the Stop
	// hook fires before CC flushes all entries to JSONL, so we retry until
	// the count stabilizes (two consecutive reads return the same count).
	var texts []string
	prevCount := -1
	for attempt := range retryAttempts {
		texts, err = extractCurrentTurnTexts(input.TranscriptPath)
		if err != nil {
			logBridge("extract error (attempt %d): %v", attempt, err)
			return nil
		}
		if len(texts) > 0 && len(texts) == prevCount {
			break // count stabilized
		}
		prevCount = len(texts)
		time.Sleep(retryDelay)
	}

	if len(texts) == 0 {
		logBridge("no fresh text after %d attempts for %s", retryAttempts, agentName)
		return nil
	}

	logBridge("sending %d messages for %s", len(texts), agentName)

	// Send each text block as a separate message to daemon
	for _, text := range texts {
		if err := daemon.Send(daemon.SendRequest{
			From:    agentName,
			Message: text,
		}); err != nil {
			logBridge("send error: %v", err)
		}
	}

	return nil
}

// resolveAgent opens the ttal database and finds an agent whose path matches cwd.
func resolveAgent(cwd string) (string, error) {
	database, err := db.New(db.DefaultPath())
	if err != nil {
		return "", err
	}
	defer database.Close() //nolint:errcheck

	ctx := context.Background()
	a, err := database.Agent.Query().
		Where(agent.Path(cwd)).
		Only(ctx)
	if err != nil {
		return "", err
	}

	return a.Name, nil
}

// extractCurrentTurnTexts reads the last N lines of the JSONL transcript and
// collects ALL assistant text blocks from the current turn (not followed by a
// stop_hook_summary).
//
// A single turn may produce multiple assistant entries (text blocks separated
// by tool calls). We return all of them in order so each can be sent as a
// separate Telegram message.
//
// Returns nil if no fresh (current-turn) assistant text is found.
func extractCurrentTurnTexts(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Take last N lines
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}

	// Parse entries from the tail
	type parsedEntry struct {
		typ     string
		subtype string
		text    string // extracted assistant text, if any
	}

	var entries []parsedEntry
	for _, line := range lines {
		var entry jsonlEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			continue
		}

		pe := parsedEntry{typ: entry.Type, subtype: entry.Subtype}

		if entry.Type == "assistant" {
			var msg assistantMessage
			if err := json.Unmarshal(entry.Message, &msg); err == nil {
				var texts []string
				for _, block := range msg.Content {
					trimmed := strings.TrimSpace(block.Text)
					if block.Type == "text" && trimmed != "" {
						texts = append(texts, trimmed)
					}
				}
				if len(texts) > 0 {
					pe.text = strings.Join(texts, "\n\n")
				}
			}
		}

		entries = append(entries, pe)
	}

	// Find the last stop_hook_summary — everything after it is the current turn.
	lastSummary := -1
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].typ == "system" && entries[i].subtype == "stop_hook_summary" {
			lastSummary = i
			break
		}
	}

	// Collect all assistant texts after the last stop_hook_summary.
	var result []string
	for i := lastSummary + 1; i < len(entries); i++ {
		if entries[i].text != "" {
			result = append(result, entries[i].text)
		}
	}

	return result, nil
}

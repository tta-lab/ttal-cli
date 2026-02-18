package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strings"

	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/internal/daemon"
	"codeberg.org/clawteam/ttal-cli/internal/db"
)

// StopHookInput is the JSON schema CC sends to Stop hooks via stdin.
type StopHookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	Cwd            string `json:"cwd"`
	StopHookActive bool   `json:"stop_hook_active"`
}

// tailLines is the number of lines to read from the end of the transcript.
const tailLines = 20

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

	// Extract last assistant text from transcript
	text, err := extractLastAssistantText(input.TranscriptPath)
	if err != nil || text == "" {
		return nil
	}

	// Send to daemon via socket — swallow errors silently
	_ = daemon.Send(daemon.SendRequest{
		From:    agentName,
		Message: text,
	})

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

// extractLastAssistantText reads the last N lines of the JSONL transcript
// and scans backwards for the last assistant entry with text content.
func extractLastAssistantText(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close() //nolint:errcheck

	// Read all lines (JSONL files are typically small per-session)
	var lines []string
	scanner := bufio.NewScanner(f)
	// Increase buffer for potentially large JSONL lines
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// Take last N lines
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}

	// Scan backwards for last assistant entry with text
	for i := len(lines) - 1; i >= 0; i-- {
		var entry jsonlEntry
		if err := json.Unmarshal([]byte(lines[i]), &entry); err != nil {
			continue
		}

		if entry.Type != "assistant" {
			continue
		}

		var msg assistantMessage
		if err := json.Unmarshal(entry.Message, &msg); err != nil {
			continue
		}

		// Collect all text blocks
		var texts []string
		for _, block := range msg.Content {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				texts = append(texts, strings.TrimSpace(block.Text))
			}
		}

		if len(texts) > 0 {
			return strings.Join(texts, "\n\n"), nil
		}
	}

	return "", nil
}

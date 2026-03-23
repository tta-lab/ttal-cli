package watcher

import (
	"encoding/json"
	"log"
	"strings"
)

const jsonlTypeAssistant = "assistant"

// jsonlEntry represents a single line in the CC session JSONL transcript.
type jsonlEntry struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// assistantMessage represents the message body of a type:"assistant" entry.
type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

// contentBlock represents a single content block in an assistant message.
type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`  // tool_use name
	ID    string          `json:"id,omitempty"`    // tool_use_id
	Input json.RawMessage `json:"input,omitempty"` // tool_use input
}

// bashInput extracts the command field from a Bash tool_use input.
type bashInput struct {
	Command string `json:"command"`
}

const toolBash = "Bash"

// refineBashTool inspects a Bash tool's input.command and returns a more
// specific tool identifier for known CLI commands. Falls back to "Bash".
// Keep the command list in sync with flicknote and ttal subcommands.
func refineBashTool(input json.RawMessage) string {
	var bi bashInput
	if err := json.Unmarshal(input, &bi); err != nil {
		log.Printf("[watcher] failed to parse Bash tool input: %v", err)
		return toolBash
	}
	if bi.Command == "" {
		return toolBash
	}

	// Normalize: trim leading whitespace
	cmd := strings.TrimSpace(bi.Command)

	// Handle pipes: "echo x | flicknote add ..." or "cat <<'EOF' | flicknote add ..."
	// Use LastIndex to find the rightmost pipe — the final command in the pipeline.
	if idx := strings.LastIndex(cmd, "| "); idx >= 0 {
		cmd = strings.TrimSpace(cmd[idx+2:])
	}

	switch {
	case strings.HasPrefix(cmd, "ttal send "):
		return "ttal:send"
	case strings.HasPrefix(cmd, "ttal go "):
		// ttal go may block on Telegram approval gate; the daemon
		// handles output. No direct Telegram output from the CLI side.
		return "ttal:route"
	case strings.HasPrefix(cmd, "flicknote add "),
		strings.HasPrefix(cmd, "flicknote replace "),
		strings.HasPrefix(cmd, "flicknote append "),
		strings.HasPrefix(cmd, "flicknote insert "),
		strings.HasPrefix(cmd, "flicknote remove "),
		strings.HasPrefix(cmd, "flicknote rename "),
		strings.HasPrefix(cmd, "flicknote archive "):
		return "flicknote:write"
	case strings.HasPrefix(cmd, "flicknote get "),
		cmd == "flicknote list",
		strings.HasPrefix(cmd, "flicknote list "):
		return "flicknote:read"
	default:
		return toolBash
	}
}

// extractToolUse detects tool_use blocks in an assistant JSONL entry.
// Returns the tool name of the first tool_use block, or "" if none found.
// For Bash tools, it inspects the command to return a refined tool identifier.
func extractToolUse(line []byte) string {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil || entry.Type != jsonlTypeAssistant {
		return ""
	}

	var msg assistantMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return ""
	}

	for _, block := range msg.Content {
		if block.Type == "tool_use" {
			if block.Name == toolBash {
				return refineBashTool(block.Input)
			}
			return block.Name
		}
	}

	return ""
}

// noisyPhrases is a list of exact texts (case-insensitive) that are CC
// internal status messages rather than meaningful agent output.  These should
// be suppressed before forwarding text to Telegram.
var noisyPhrases = []string{
	"no response requested",
}

// isNoisyText reports whether text is a known CC noise phrase that should be
// suppressed before forwarding to Telegram.
func isNoisyText(text string) bool {
	lower := strings.ToLower(strings.TrimRight(strings.TrimSpace(text), "."))
	for _, phrase := range noisyPhrases {
		if lower == phrase {
			return true
		}
	}
	return false
}

// extractAssistantText parses a JSONL line and returns the assistant text
// if it's a type=assistant entry with text content blocks. Returns "" otherwise.
func extractAssistantText(line []byte) string {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	if entry.Type != jsonlTypeAssistant {
		return ""
	}

	var msg assistantMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return ""
	}

	var texts []string
	for _, block := range msg.Content {
		if block.Type == "text" {
			trimmed := strings.TrimSpace(block.Text)
			if trimmed != "" {
				texts = append(texts, trimmed)
			}
		}
	}

	if len(texts) == 0 {
		return ""
	}

	return strings.Join(texts, "\n\n")
}

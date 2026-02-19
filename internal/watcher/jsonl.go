package watcher

import (
	"encoding/json"
	"strings"
)

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
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// extractAssistantText parses a JSONL line and returns the assistant text
// if it's a type=assistant entry with text content blocks. Returns "" otherwise.
func extractAssistantText(line []byte) string {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	if entry.Type != "assistant" {
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

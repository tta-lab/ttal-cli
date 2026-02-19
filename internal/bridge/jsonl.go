package bridge

import "encoding/json"

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

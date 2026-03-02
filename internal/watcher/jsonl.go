package watcher

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/runtime"
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
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`  // tool_use name
	ID    string          `json:"id,omitempty"`    // tool_use_id
	Input json.RawMessage `json:"input,omitempty"` // tool_use input
}

// askUserInput is the input schema for AskUserQuestion tool_use blocks.
type askUserInput struct {
	Questions []askUserQuestion `json:"questions"`
}

type askUserQuestion struct {
	Question    string          `json:"question"`
	Header      string          `json:"header"`
	Options     []askUserOption `json:"options"`
	MultiSelect bool            `json:"multiSelect"`
}

type askUserOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

// extractQuestions detects AskUserQuestion tool_use blocks in an assistant JSONL entry.
// Returns empty correlationID and nil if no questions found.
func extractQuestions(line []byte) (correlationID string, questions []runtime.Question) {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil || entry.Type != "assistant" {
		return "", nil
	}

	var msg assistantMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return "", nil
	}

	for _, block := range msg.Content {
		if block.Type != "tool_use" || block.Name != "AskUserQuestion" {
			continue
		}

		var input askUserInput
		if err := json.Unmarshal(block.Input, &input); err != nil {
			log.Printf("[watcher] failed to parse AskUserQuestion input: %v", err)
			continue
		}

		for _, q := range input.Questions {
			rq := runtime.Question{
				Header:      q.Header,
				Text:        q.Question,
				MultiSelect: q.MultiSelect,
				AllowCustom: true, // CC always allows custom via implicit "Other"
			}
			for _, opt := range q.Options {
				rq.Options = append(rq.Options, runtime.QuestionOption{
					Label:       opt.Label,
					Description: opt.Description,
				})
			}
			questions = append(questions, rq)
		}

		return block.ID, questions
	}

	return "", nil
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

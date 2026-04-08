package watcher

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/cmdexec"
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
// Keep the command list in sync with flicknote and ttal subcommands
// (maintained in cmdexec.ClassifyShellCmd).
func refineBashTool(input json.RawMessage) string {
	var bi bashInput
	if err := json.Unmarshal(input, &bi); err != nil {
		log.Printf("[watcher] failed to parse Bash tool input: %v", err)
		return toolBash
	}
	if bi.Command == "" {
		return toolBash
	}
	return cmdexec.ClassifyShellCmd(bi.Command)
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

// ParseCmdBlocks extracts the contents of all <cmd>...</cmd> blocks from text,
// in document order. Each block's content has whitespace trimmed.
// Nested blocks are preserved as-is (content is not further split).
// Returns an empty slice if no blocks are found.
func ParseCmdBlocks(text string) []string {
	var cmds []string
	open := "<cmd>"
	close := "</cmd>"
	depth := 0
	searchFrom := 0
	blockStart := -1

	for {
		if depth == 0 {
			idx := strings.Index(text[searchFrom:], open)
			if idx < 0 {
				break
			}
			depth = 1
			blockStart = searchFrom + idx + len(open)
			searchFrom = blockStart
		} else {
			nextOpen := strings.Index(text[searchFrom:], open)
			nextClose := strings.Index(text[searchFrom:], close)

			if nextClose < 0 {
				break // unclosed block
			}
			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				searchFrom = searchFrom + nextOpen + len(open)
			} else {
				depth--
				if depth == 0 {
					content := strings.TrimSpace(text[blockStart : searchFrom+nextClose])
					cmds = append(cmds, content)
					searchFrom = searchFrom + nextClose + len(close)
				} else {
					// Still inside a block — keep searching for the outer close.
					searchFrom = searchFrom + nextClose + len(close)
				}
			}
		}
	}
	return cmds
}

// StripCmdBlocks removes all <cmd>...</cmd> blocks from text and returns
// the remaining prose. Concatenates prose fragments with a single blank line.
func StripCmdBlocks(text string) string {
	open := "<cmd>"
	close := "</cmd>"
	var prose []string
	depth := 0
	searchFrom := 0

	for {
		if depth == 0 {
			idx := strings.Index(text[searchFrom:], open)
			if idx < 0 {
				break
			}
			fragment := strings.TrimSpace(text[searchFrom : searchFrom+idx])
			if fragment != "" {
				prose = append(prose, fragment)
			}
			depth = 1
			searchFrom = searchFrom + idx + len(open)
		} else {
			nextOpen := strings.Index(text[searchFrom:], open)
			nextClose := strings.Index(text[searchFrom:], close)

			if nextClose < 0 {
				break
			}
			if nextOpen >= 0 && nextOpen < nextClose {
				depth++
				searchFrom = searchFrom + nextOpen + len(open)
			} else {
				depth--
				if depth == 0 {
					searchFrom = searchFrom + nextClose + len(close)
				} else {
					searchFrom = searchFrom + nextClose + len(close)
				}
			}
		}
	}

	// Append remaining tail after last closing tag.
	if tail := strings.TrimSpace(text[searchFrom:]); tail != "" {
		prose = append(prose, tail)
	}

	return strings.Join(prose, "\n\n")
}

// extractAssistantTextAndCmds parses a JSONL line and returns the prose-stripped
// assistant text and the extracted <cmd> block contents.
// Prose is empty if the assistant message contains only cmd blocks.
// Cmds is empty if no cmd blocks are found.
func extractAssistantTextAndCmds(line []byte) (prose string, cmds []string) {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil || entry.Type != jsonlTypeAssistant {
		return "", nil
	}

	var msg assistantMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return "", nil
	}

	// Join all text blocks.
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
		return "", nil
	}

	joined := strings.Join(texts, "\n\n")
	cmds = ParseCmdBlocks(joined)
	prose = StripCmdBlocks(joined)
	return prose, cmds
}

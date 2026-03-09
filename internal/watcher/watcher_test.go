package watcher

import (
	"encoding/json"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestEncodePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{"absolute path", "/Users/neil/clawd", "-Users-neil-clawd"},
		{"path with dots", "/Users/neil/my.project", "-Users-neil-my-project"},
		{"root", "/", "-"},
		{"no separators", "simple", "simple"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EncodePath(tt.path)
			if got != tt.want {
				t.Errorf("EncodePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSplitCompleteLines(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  int
	}{
		{"empty", []byte{}, 0},
		{"single complete line", []byte("hello\n"), 1},
		{"trailing partial", []byte("hello\nworld"), 1},
		{"two complete lines", []byte("a\nb\n"), 2},
		{"only partial", []byte("no newline"), 0},
		{"empty lines", []byte("\n\n"), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitCompleteLines(tt.input)
			if len(got) != tt.want {
				t.Errorf("splitCompleteLines(%q) returned %d lines, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestExtractQuestions(t *testing.T) {
	t.Run("single question with options", func(t *testing.T) {
		input := map[string]interface{}{
			"questions": []map[string]interface{}{
				{
					"question":    "Which database?",
					"header":      "Database",
					"multiSelect": false,
					"options": []map[string]string{
						{"label": "PostgreSQL", "description": "Relational DB"},
						{"label": "MongoDB", "description": "Document DB"},
					},
				},
			},
		}
		correlationID, questions := extractQuestionsFromInput(t, input, "toolu_123")
		assertCorrelationID(t, correlationID, "toolu_123")
		if len(questions) != 1 {
			t.Fatalf("len(questions) = %d, want 1", len(questions))
		}
		q := questions[0]
		assertQuestionFields(t, q, "Which database?", "Database", 2, true)
	})

	t.Run("multi question batch", func(t *testing.T) {
		input := map[string]interface{}{
			"questions": []map[string]interface{}{
				{"question": "Q1", "header": "H1", "options": []map[string]string{{"label": "A"}}},
				{"question": "Q2", "header": "H2", "options": []map[string]string{{"label": "B"}}},
			},
		}
		correlationID, questions := extractQuestionsFromInput(t, input, "toolu_456")
		assertCorrelationID(t, correlationID, "toolu_456")
		if len(questions) != 2 {
			t.Fatalf("len(questions) = %d, want 2", len(questions))
		}
	})

	t.Run("non-assistant type returns nil", func(t *testing.T) {
		line := `{"type":"user","message":{"content":[{"type":"tool_use",` +
			`"id":"x","name":"AskUserQuestion","input":{"questions":[]}}]}}`
		assertEmptyExtraction(t, line, "non-assistant")
	})

	t.Run("non-AskUserQuestion tool_use ignored", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[` +
			`{"type":"tool_use","id":"x","name":"Read","input":{}}]}}`
		assertEmptyExtraction(t, line, "non-question tool_use")
	})

	t.Run("text-only assistant returns nil", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[` +
			`{"type":"text","text":"hello"}]}}`
		assertEmptyExtraction(t, line, "text-only")
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		assertEmptyExtraction(t, "not json", "invalid json")
	})
}

// extractQuestionsFromInput builds a JSONL line from input and toolID, then calls extractQuestions.
func extractQuestionsFromInput(
	t *testing.T, input map[string]interface{}, toolID string,
) (string, []runtime.Question) {
	t.Helper()
	inputJSON, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("failed to marshal input: %v", err)
	}
	line := `{"type":"assistant","message":{"content":[{"type":"tool_use",` +
		`"id":"` + toolID + `","name":"AskUserQuestion","input":` +
		string(inputJSON) + `}]}}`
	return extractQuestions([]byte(line))
}

func assertCorrelationID(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("correlationID = %q, want %q", got, want)
	}
}

func assertQuestionFields(
	t *testing.T, q runtime.Question,
	wantText, wantHeader string, wantOptions int, wantCustom bool,
) {
	t.Helper()
	if q.Text != wantText {
		t.Errorf("Text = %q, want %q", q.Text, wantText)
	}
	if q.Header != wantHeader {
		t.Errorf("Header = %q, want %q", q.Header, wantHeader)
	}
	if len(q.Options) != wantOptions {
		t.Errorf("len(Options) = %d, want %d", len(q.Options), wantOptions)
	}
	if q.AllowCustom != wantCustom {
		t.Errorf("AllowCustom = %v, want %v", q.AllowCustom, wantCustom)
	}
}

func assertEmptyExtraction(t *testing.T, line, desc string) {
	t.Helper()
	correlationID, questions := extractQuestions([]byte(line))
	if correlationID != "" || questions != nil {
		t.Errorf("expected empty result for %s, got %q %v",
			desc, correlationID, questions)
	}
}

func TestExtractToolUse(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		wantTool string
	}{
		{
			name: "Read tool",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Read","id":"tu_1","input":{}}]}}`,
			wantTool: "Read",
		},
		{
			name: "Bash tool",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_2","input":{}}]}}`,
			wantTool: "Bash",
		},
		{
			name: "text block returns empty",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"text","text":"hello"}]}}`,
			wantTool: "",
		},
		{
			name: "AskUserQuestion skipped",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"AskUserQuestion","id":"tu_3","input":{}}]}}`,
			wantTool: "",
		},
		{
			name:     "non-assistant type",
			line:     `{"type":"human","message":{"content":[{"type":"text","text":"hi"}]}}`,
			wantTool: "",
		},
		{
			name: "Bash with flicknote add",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_4",` +
				`"input":{"command":"flicknote add \"content\" --project ttal.plans"}}]}}`,
			wantTool: "flicknote:write",
		},
		{
			name: "Bash with ttal send",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_5","input":{"command":"ttal send --to yuki \"hello\""}}]}}`,
			wantTool: "ttal:send",
		},
		{
			name: "Bash with flicknote get",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_6","input":{"command":"flicknote get abc123"}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with heredoc pipe to flicknote",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_7",` +
				`"input":{"command":"cat <<'EOF' | flicknote add --project ttal.plans\ncontent\nEOF"}}]}}`,
			wantTool: "flicknote:write",
		},
		{
			name: "Bash with echo pipe to flicknote replace",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_8",` +
				`"input":{"command":"echo \"new content\" | flicknote replace abc123"}}]}}`,
			wantTool: "flicknote:write",
		},
		{
			name: "Bash with ttal task route",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_9","input":{"command":"ttal task route abc123 --to inke"}}]}}`,
			wantTool: "ttal:route",
		},
		{
			name: "Bash with unknown command",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_10","input":{"command":"git status"}}]}}`,
			wantTool: "Bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractToolUse([]byte(tt.line))
			if got != tt.wantTool {
				t.Errorf("extractToolUse() = %q, want %q", got, tt.wantTool)
			}
		})
	}
}

func TestExtractAssistantText(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			"assistant with text",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}`,
			"hello world",
		},
		{
			"multiple text blocks",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}}`,
			"first\n\nsecond",
		},
		{
			"non-assistant type",
			`{"type":"user","message":{"content":[{"type":"text","text":"hello"}]}}`,
			"",
		},
		{
			"assistant with tool_use only",
			`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"123"}]}}`,
			"",
		},
		{
			"empty text trimmed",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"  "}]}}`,
			"",
		},
		{
			"text gets trimmed",
			`{"type":"assistant","message":{"content":[{"type":"text","text":"  hello  "}]}}`,
			"hello",
		},
		{
			"invalid json",
			`not json`,
			"",
		},
		{
			"empty content array",
			`{"type":"assistant","message":{"content":[]}}`,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractAssistantText([]byte(tt.line))
			if got != tt.want {
				t.Errorf("extractAssistantText() = %q, want %q", got, tt.want)
			}
		})
	}
}

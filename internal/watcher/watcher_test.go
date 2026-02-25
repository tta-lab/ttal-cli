package watcher

import (
	"encoding/json"
	"testing"
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
			got := encodePath(tt.path)
			if got != tt.want {
				t.Errorf("encodePath(%q) = %q, want %q", tt.path, got, tt.want)
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
		inputJSON, _ := json.Marshal(input)
		line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_123","name":"AskUserQuestion","input":` + string(inputJSON) + `}]}}`

		correlationID, questions := extractQuestions([]byte(line))
		if correlationID != "toolu_123" {
			t.Errorf("correlationID = %q, want %q", correlationID, "toolu_123")
		}
		if len(questions) != 1 {
			t.Fatalf("len(questions) = %d, want 1", len(questions))
		}
		q := questions[0]
		if q.Text != "Which database?" {
			t.Errorf("Text = %q, want %q", q.Text, "Which database?")
		}
		if q.Header != "Database" {
			t.Errorf("Header = %q, want %q", q.Header, "Database")
		}
		if len(q.Options) != 2 {
			t.Errorf("len(Options) = %d, want 2", len(q.Options))
		}
		if !q.AllowCustom {
			t.Error("AllowCustom should be true for CC questions")
		}
	})

	t.Run("multi question batch", func(t *testing.T) {
		input := map[string]interface{}{
			"questions": []map[string]interface{}{
				{"question": "Q1", "header": "H1", "options": []map[string]string{{"label": "A"}}},
				{"question": "Q2", "header": "H2", "options": []map[string]string{{"label": "B"}}},
			},
		}
		inputJSON, _ := json.Marshal(input)
		line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_456","name":"AskUserQuestion","input":` + string(inputJSON) + `}]}}`

		correlationID, questions := extractQuestions([]byte(line))
		if correlationID != "toolu_456" {
			t.Errorf("correlationID = %q, want %q", correlationID, "toolu_456")
		}
		if len(questions) != 2 {
			t.Fatalf("len(questions) = %d, want 2", len(questions))
		}
	})

	t.Run("non-assistant type returns nil", func(t *testing.T) {
		line := `{"type":"user","message":{"content":[{"type":"tool_use","id":"x","name":"AskUserQuestion","input":{"questions":[]}}]}}`
		correlationID, questions := extractQuestions([]byte(line))
		if correlationID != "" || questions != nil {
			t.Errorf("expected empty result for non-assistant, got %q %v", correlationID, questions)
		}
	})

	t.Run("non-AskUserQuestion tool_use ignored", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"x","name":"Read","input":{}}]}}`
		correlationID, questions := extractQuestions([]byte(line))
		if correlationID != "" || questions != nil {
			t.Errorf("expected empty result for non-question tool_use, got %q %v", correlationID, questions)
		}
	})

	t.Run("text-only assistant returns nil", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`
		correlationID, questions := extractQuestions([]byte(line))
		if correlationID != "" || questions != nil {
			t.Errorf("expected empty result for text-only, got %q %v", correlationID, questions)
		}
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		correlationID, questions := extractQuestions([]byte("not json"))
		if correlationID != "" || questions != nil {
			t.Errorf("expected empty result for invalid json")
		}
	})
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

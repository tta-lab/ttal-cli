package watcher

import (
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
			name: "Bash with ttal task advance",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_9","input":{"command":"ttal task advance abc12345"}}]}}`,
			wantTool: "ttal:route",
		},
		{
			name: "Bash with unknown command",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_10","input":{"command":"git status"}}]}}`,
			wantTool: "Bash",
		},
		{
			name: "Bash with ttal task design",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_11","input":{"command":"ttal task design abc123"}}]}}`,
			wantTool: "ttal:route",
		},
		{
			name: "Bash with ttal task research",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_12","input":{"command":"ttal task research abc123"}}]}}`,
			wantTool: "ttal:route",
		},
		{
			name: "Bash with bare flicknote list",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_13","input":{"command":"flicknote list"}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with flicknote list --project",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_14","input":{"command":"flicknote list --project ttal"}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with multi-pipe command",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_15",` +
				`"input":{"command":"cat x | grep y | flicknote add --project foo"}}]}}`,
			wantTool: "flicknote:write",
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

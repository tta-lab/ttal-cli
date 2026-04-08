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
			name: "Bash with flicknote detail",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_6","input":{"command":"flicknote detail abc123"}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with flicknote content",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_6b","input":{"command":"flicknote content abc123"}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with flicknote find",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_6c","input":{"command":"flicknote find \"keyword\""}}]}}`,
			wantTool: "flicknote:read",
		},
		{
			name: "Bash with flicknote count",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_6d","input":{"command":"flicknote count --project ttal"}}]}}`,
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
			name: "Bash with echo pipe to flicknote modify",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_8",` +
				`"input":{"command":"echo \"new content\" | flicknote modify abc123"}}]}}`,
			wantTool: "flicknote:write",
		},
		{
			name: "Bash with ttal go",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_9b","input":{"command":"ttal go abc12345"}}]}}`,
			wantTool: "ttal:route",
		},
		{
			name: "Bash with unknown command",
			line: `{"type":"assistant","message":{"content":` +
				`[{"type":"tool_use","name":"Bash","id":"tu_10","input":{"command":"git status"}}]}}`,
			wantTool: "Bash",
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

func TestIsNoisyText(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		noisy bool
	}{
		{"exact phrase", "No response requested", true},
		{"with period", "No response requested.", true},
		{"lowercase", "no response requested", true},
		{"with extra whitespace", "  No response requested.  ", true},
		{"meaningful text", "Task complete!", false},
		{"empty string", "", false},
		{"partial match", "No response requested but also something else", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNoisyText(tt.text)
			if got != tt.noisy {
				t.Errorf("isNoisyText(%q) = %v, want %v", tt.text, got, tt.noisy)
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

func TestProcessLinesFiltersNoisyText(t *testing.T) {
	noisyLine := `{"type":"assistant","message":{"content":[{"type":"text","text":"No response requested."}]}}` + "\n"
	meaningfulLine := `{"type":"assistant","message":{"content":[{"type":"text","text":"Task complete!"}]}}` + "\n"

	var sent []string
	w := &Watcher{
		send: func(_, _, text string) { sent = append(sent, text) },
	}

	w.processLines([]byte(noisyLine), AgentInfo{TeamName: "t", AgentName: "a"})
	if len(sent) != 0 {
		t.Errorf("noisy text should be suppressed, got %v", sent)
	}

	w.processLines([]byte(meaningfulLine), AgentInfo{TeamName: "t", AgentName: "a"})
	if len(sent) != 1 || sent[0] != "Task complete!" {
		t.Errorf("meaningful text should be forwarded, got %v", sent)
	}
}

func TestParseCmdBlocks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "single block",
			text: "hello <cmd>ls -la</cmd> world",
			want: []string{"ls -la"},
		},
		{
			name: "multiple blocks",
			text: "<cmd>echo a</cmd><cmd>echo b</cmd>",
			want: []string{"echo a", "echo b"},
		},
		{
			name: "nested blocks",
			text: "<cmd>outer <cmd>inner</cmd> back</cmd>",
			want: []string{"outer <cmd>inner</cmd> back"},
		},
		{
			name: "prose only",
			text: "just prose here",
			want: nil,
		},
		{
			name: "multiline block",
			text: "<cmd>set -e\ncd /tmp\nls</cmd>",
			want: []string{"set -e\ncd /tmp\nls"},
		},
		{
			name: "empty block",
			text: "<cmd></cmd>",
			want: []string{""},
		},
		{
			name: "prose around block",
			text: "start <cmd>ls</cmd> end",
			want: []string{"ls"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCmdBlocks(tt.text)
			if len(got) != len(tt.want) {
				t.Errorf("ParseCmdBlocks(%q) = %v, want %v", tt.text, got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseCmdBlocks[%d](%q) = %q, want %q", i, tt.text, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestStripCmdBlocks(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "strip single block",
			text: "hello <cmd>ls</cmd> world",
			want: "hello\n\nworld",
		},
		{
			name: "strip multiple blocks",
			text: "before <cmd>echo a</cmd> middle <cmd>echo b</cmd> after",
			want: "before\n\nmiddle\n\nafter",
		},
		{
			name: "only prose",
			text: "just prose",
			want: "just prose",
		},
		{
			name: "only cmd",
			text: "<cmd>ls</cmd>",
			want: "",
		},
		{
			name: "multiline prose",
			text: "line1\nline2 <cmd>ls</cmd> line3\nline4",
			want: "line1\nline2\n\nline3\nline4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StripCmdBlocks(tt.text)
			if got != tt.want {
				t.Errorf("StripCmdBlocks(%q) = %q, want %q", tt.text, got, tt.want)
			}
		})
	}
}

func TestExtractAssistantTextAndCmds(t *testing.T) {
	// Assistant line with only prose → send called, onCmd NOT called.
	t.Run("prose only", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"hello world"}]}}`
		prose, cmds := extractAssistantTextAndCmds([]byte(line))
		if prose != "hello world" {
			t.Errorf("prose = %q, want %q", prose, "hello world")
		}
		if len(cmds) != 0 {
			t.Errorf("cmds = %v, want nil", cmds)
		}
	})

	// Assistant line with one cmd block.
	t.Run("one cmd block", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"<cmd>ls -la</cmd>"}]}}`
		prose, cmds := extractAssistantTextAndCmds([]byte(line))
		if prose != "" {
			t.Errorf("prose = %q, want %q", prose, "")
		}
		if len(cmds) != 1 || cmds[0] != "ls -la" {
			t.Errorf("cmds = %v, want [ls -la]", cmds)
		}
	})

	// Assistant line with multiple cmd blocks.
	t.Run("multiple cmd blocks", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"<cmd>echo a</cmd><cmd>echo b</cmd>"}]}}`
		prose, cmds := extractAssistantTextAndCmds([]byte(line))
		if prose != "" {
			t.Errorf("prose = %q, want %q", prose, "")
		}
		if len(cmds) != 2 || cmds[0] != "echo a" || cmds[1] != "echo b" {
			t.Errorf("cmds = %v, want [echo a, echo b]", cmds)
		}
	})

	// Assistant line with prose + cmd → prose is stripped.
	t.Run("prose plus cmd stripped", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"done <cmd>git status</cmd>"}]}}`
		prose, cmds := extractAssistantTextAndCmds([]byte(line))
		if prose != "done" {
			t.Errorf("prose = %q, want %q", prose, "done")
		}
		if len(cmds) != 1 || cmds[0] != "git status" {
			t.Errorf("cmds = %v, want [git status]", cmds)
		}
	})
}

func TestProcessLinesCmdExtraction(t *testing.T) {
	// Assistant line with only cmd blocks → send NOT called, onCmd called once with full slice.
	t.Run("cmd only fires cmdFunc", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"<cmd>ls</cmd>"}]}}` + "\n"
		var sentText []string
		var cmdCalls [][]string
		w := &Watcher{
			send: func(_, _, text string) { sentText = append(sentText, text) },
			cmdFunc: func(_, _ string, cmds []string) { cmdCalls = append(cmdCalls, cmds) },
		}
		w.processLines([]byte(line), AgentInfo{TeamName: "t", AgentName: "a"})

		if len(sentText) != 0 {
			t.Errorf("send should not be called for cmd-only, got texts: %v", sentText)
		}
		if len(cmdCalls) != 1 {
			t.Errorf("cmdFunc called %d times, want 1", len(cmdCalls))
		}
		if len(cmdCalls[0]) != 1 || cmdCalls[0][0] != "ls" {
			t.Errorf("cmds = %v, want [[ls]]", cmdCalls)
		}
	})

	// Multiple blocks → single cmdFunc call with full slice.
	t.Run("multiple blocks single cmdFunc call", func(t *testing.T) {
		line := `{"type":"assistant","message":{"content":[{"type":"text","text":"<cmd>echo a</cmd><cmd>echo b</cmd>"}]}}` + "\n"
		var cmdCalls [][]string
		w := &Watcher{
			cmdFunc: func(_, _ string, cmds []string) { cmdCalls = append(cmdCalls, cmds) },
		}
		w.processLines([]byte(line), AgentInfo{TeamName: "t", AgentName: "a"})
		if len(cmdCalls) != 1 {
			t.Errorf("cmdFunc called %d times, want 1", len(cmdCalls))
		}
		if len(cmdCalls[0]) != 2 {
			t.Errorf("cmds[0] len = %d, want 2: %v", len(cmdCalls[0]), cmdCalls[0])
		}
	})
}

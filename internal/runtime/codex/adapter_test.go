package codex

import (
	"encoding/json"
	"testing"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func TestProcessNotificationItemStarted(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		wantTool string
	}{
		{
			name:     "command_execution",
			params:   `{"type":"command_execution","command":"ls","cwd":"/tmp"}`,
			wantTool: "Bash",
		},
		{
			name:     "file_change",
			params:   `{"type":"file_change","changes":[]}`,
			wantTool: "Edit",
		},
		{
			name:     "mcp_tool_call",
			params:   `{"type":"mcp_tool_call","server":"ctx7","tool":"query"}`,
			wantTool: "mcp_tool_call",
		},
		{
			name:     "web_search",
			params:   `{"type":"web_search","query":"golang"}`,
			wantTool: "WebSearch",
		},
		{
			name:     "reasoning",
			params:   `{"type":"reasoning","summary":"thinking"}`,
			wantTool: "reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Adapter{
				cfg:    runtime.AdapterConfig{AgentName: "test"},
				events: make(chan runtime.Event, 16),
			}

			a.processNotification(rpcResponse{
				Method: "item/started",
				Params: json.RawMessage(tt.params),
			})

			select {
			case evt := <-a.events:
				if evt.Type != runtime.EventTool {
					t.Errorf("got event type %q, want %q", evt.Type, runtime.EventTool)
				}
				if evt.ToolName != tt.wantTool {
					t.Errorf("got tool name %q, want %q", evt.ToolName, tt.wantTool)
				}
			default:
				t.Error("no event emitted")
			}
		})
	}
}

func TestCodexItemToToolName(t *testing.T) {
	tests := []struct {
		itemType string
		want     string
	}{
		{"command_execution", "Bash"},
		{"file_change", "Edit"},
		{"web_search", "WebSearch"},
		{"mcp_tool_call", "mcp_tool_call"},
		{"reasoning", "reasoning"},
		{"unknown_type", "unknown_type"},
	}

	for _, tt := range tests {
		t.Run(tt.itemType, func(t *testing.T) {
			got := codexItemToToolName(tt.itemType)
			if got != tt.want {
				t.Errorf("codexItemToToolName(%q) = %q, want %q", tt.itemType, got, tt.want)
			}
		})
	}
}

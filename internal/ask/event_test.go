package ask

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEventJSON(t *testing.T) {
	tests := []struct {
		name  string
		event Event
	}{
		{
			name:  "delta",
			event: Event{Type: EventDelta, Text: "hello world"},
		},
		{
			name:  "command_start",
			event: Event{Type: EventCommandStart, Command: "rg pattern"},
		},
		{
			name:  "command_result",
			event: Event{Type: EventCommandResult, Command: "rg pattern", Output: "match1\nmatch2", ExitCode: 0},
		},
		{
			name:  "retry",
			event: Event{Type: EventRetry, Reason: "tool_call", Step: 3},
		},
		{
			name:  "status",
			event: Event{Type: EventStatus, Message: "Cloning repo..."},
		},
		{
			name:  "done",
			event: Event{Type: EventDone, Response: "the final answer"},
		},
		{
			name:  "error",
			event: Event{Type: EventError, Message: "max steps reached"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.event)
			require.NoError(t, err)

			var got Event
			err = json.Unmarshal(data, &got)
			require.NoError(t, err)
			assert.Equal(t, tt.event, got)
		})
	}
}

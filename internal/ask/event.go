package ask

// EventType identifies the kind of streaming event.
type EventType string

const (
	EventDelta         EventType = "delta"
	EventCommandStart  EventType = "command_start"
	EventCommandResult EventType = "command_result"
	EventRetry         EventType = "retry"
	EventStatus        EventType = "status"
	EventDone          EventType = "done"
	EventError         EventType = "error"
)

// Event is a single NDJSON streaming event from the daemon's /ask endpoint.
type Event struct {
	Type     EventType `json:"type"`
	Text     string    `json:"text,omitempty"`      // delta text
	Command  string    `json:"command,omitempty"`   // command_start / command_result
	Output   string    `json:"output,omitempty"`    // command_result output
	ExitCode int       `json:"exit_code,omitempty"` // command_result exit code
	Reason   string    `json:"reason,omitempty"`    // retry reason
	Step     int       `json:"step,omitempty"`      // retry step number
	Message  string    `json:"message,omitempty"`   // status/error message
	Response string    `json:"response,omitempty"`  // done — full accumulated response
}

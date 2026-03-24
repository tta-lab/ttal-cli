package runtime

import "context"

// Event represents an output event from any runtime.
type Event struct {
	Type  EventType
	Agent string
	Text  string // assistant output text (for EventText/EventError)

	// Status fields (for EventStatus).
	ContextUsedPct      float64
	ContextRemainingPct float64
	ModelID             string
	SessionID           string

	// Tool fields (for EventTool).
	ToolName string
}

// EventType classifies runtime events.
type EventType string

const (
	EventText   EventType = "text"   // Assistant text output → bridge to Telegram
	EventStatus EventType = "status" // Context/model status update
	EventError  EventType = "error"  // Runtime error
	EventIdle   EventType = "idle"   // Agent finished processing, waiting for input
	EventTool   EventType = "tool"   // Tool invocation detected
)

// Adapter abstracts the transport for communicating with a coding agent runtime.
// Each runtime (CC, Codex) implements this interface.
type Adapter interface {
	// Start launches the runtime process and establishes connection.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the runtime.
	Stop(ctx context.Context) error

	// SendMessage delivers text to the agent.
	SendMessage(ctx context.Context, text string) error

	// Events returns a channel that receives agent output events.
	// Channel is closed when the adapter stops or the context is cancelled.
	Events() <-chan Event

	// CreateSession starts a new conversation. Returns session ID.
	CreateSession(ctx context.Context) (string, error)

	// ResumeSession resumes an existing conversation by ID.
	// Returns the session ID on success.
	ResumeSession(ctx context.Context, sessionID string) (string, error)

	// IsHealthy returns true if the runtime is responsive.
	IsHealthy(ctx context.Context) bool

	// Runtime returns the runtime type.
	Runtime() Runtime
}

// AdapterConfig holds common configuration for all adapters.
type AdapterConfig struct {
	AgentName string
	WorkDir   string   // Agent workspace directory
	Model     string   // Model override
	Env       []string // Additional env vars (TTAL_AGENT_NAME, TASKRC, etc.)
}

package daemon

// AskHumanRequest is the CLI → daemon POST body for POST /ask/human.
type AskHumanRequest struct {
	Question  string   `json:"question"`
	Options   []string `json:"options,omitempty"`
	AgentName string   `json:"agent_name,omitempty"` // from TTAL_AGENT_NAME
	Session   string   `json:"session,omitempty"`    // from tmux session name
}

// AskHumanResponse is the daemon → CLI JSON response for /ask/human.
type AskHumanResponse struct {
	OK      bool   `json:"ok"`
	Answer  string `json:"answer,omitempty"`
	Skipped bool   `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Server-side ask-human logic has moved to internal/frontend/telegram_ask.go.
// AskHumanRequest, AskHumanResponse, and the client-side AskHuman() function
// (in socket.go) remain here for the CLI → daemon wire protocol.

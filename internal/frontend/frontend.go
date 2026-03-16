package frontend

import (
	"context"
	"net/http"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
)

// Frontend abstracts a messaging transport (Telegram, Matrix).
// The daemon initialises one Frontend per team at startup.
type Frontend interface {
	// Start begins polling/syncing for inbound messages.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the frontend.
	Stop(ctx context.Context) error

	// SendText sends a text message to an agent's chat/room.
	SendText(ctx context.Context, agentName string, text string) error

	// SendVoice sends voice audio to an agent's chat/room.
	SendVoice(ctx context.Context, agentName string, data []byte) error

	// SendNotification sends a system notification (daemon ready, task done, etc).
	// Routes to the team's notification channel.
	SendNotification(ctx context.Context, text string) error

	// SetReaction sets an emoji reaction on the last inbound message for an agent.
	SetReaction(ctx context.Context, agentName string, emoji string) error

	// AskHuman sends a question and blocks until answered or timed out.
	// If options is non-empty, presents them as choices.
	// Returns the human's answer, or skipped=true on timeout/skip.
	AskHuman(ctx context.Context, agentName, question string, options []string) (answer string, skipped bool, err error)

	// AskHumanHTTPHandler returns an http.HandlerFunc for POST /ask/human.
	// daemon.go wires this into httpHandlers.askHuman at startup.
	AskHumanHTTPHandler() http.HandlerFunc

	// RegisterCommands registers bot commands for discoverability and stores them
	// for use by the polling handlers. Must be called before Start.
	RegisterCommands(commands []Command) error

	// StartNotificationPoller starts the notification-only command handler.
	StartNotificationPoller(ctx context.Context) error
}

// Command describes a bot command for registration and handler dispatch.
type Command struct {
	Name         string // Telegram command name (sanitized: only [a-z0-9_])
	Description  string
	OriginalName string // original hyphenated name for agent dispatch (e.g. "review-pr")
}

// InboundHandler is called when a message arrives from the human.
// The frontend formats the message with the correct transport prefix before calling this.
// e.g. "[telegram from:neil] hello" or "[matrix from:neil] hello".
type InboundHandler func(teamName, agentName, text string)

// TelegramConfig holds construction parameters for TelegramFrontend.
type TelegramConfig struct {
	TeamName   string
	MCfg       *config.DaemonConfig
	OnMessage  InboundHandler
	MsgSvc     *message.Service
	UserNameFn func() string // returns human display name for this team
	GetUsageFn func() string // returns formatted usage string, or "" if unavailable
	RestartFn  func() error  // triggers daemon restart (launchctl kickstart -k)
}

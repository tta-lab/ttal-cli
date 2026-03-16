package frontend

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
)

// MatrixConfig holds construction parameters for MatrixFrontend.
type MatrixConfig struct {
	TeamName   string
	MCfg       *config.DaemonConfig
	OnMessage  InboundHandler
	MsgSvc     *message.Service
	UserNameFn func() string // returns human display name for this team
}

// agentSession holds a Matrix client and its associated room for one agent.
type agentSession struct {
	client *mautrix.Client
	roomID id.RoomID
}

// MatrixFrontend implements Frontend using the Matrix protocol via mautrix-go.
type MatrixFrontend struct {
	cfg          MatrixConfig
	sessions     map[string]agentSession // agentName → session
	notifyClient *mautrix.Client
	notifyRoom   id.RoomID
	cancel       context.CancelFunc
	stopOnce     sync.Once

	// Track last event ID per agent for future reaction support (Phase 4).
	lastEventMu sync.RWMutex
	lastEventID map[string]id.EventID // agentName → last inbound event ID
}

// NewMatrix constructs a MatrixFrontend from the given config.
// Returns an error if required config fields are missing or callbacks are nil.
// Agents whose env-var tokens are unset are skipped with a warning (partial setup is OK).
func NewMatrix(cfg MatrixConfig) (*MatrixFrontend, error) {
	if cfg.OnMessage == nil {
		return nil, fmt.Errorf("MatrixConfig.OnMessage is required")
	}
	if cfg.UserNameFn == nil {
		return nil, fmt.Errorf("MatrixConfig.UserNameFn is required")
	}

	team, ok := cfg.MCfg.Teams[cfg.TeamName]
	if !ok {
		return nil, fmt.Errorf("team %q not found in config", cfg.TeamName)
	}
	matrixCfg := team.Matrix
	if matrixCfg == nil {
		return nil, fmt.Errorf("team %q has frontend=matrix but no [teams.%s.matrix] config", cfg.TeamName, cfg.TeamName)
	}
	if err := matrixCfg.Validate(); err != nil {
		return nil, fmt.Errorf("team %q matrix config invalid: %w", cfg.TeamName, err)
	}

	homeserver := matrixCfg.Homeserver
	domain, err := extractDomain(homeserver)
	if err != nil {
		return nil, fmt.Errorf("team %q: invalid matrix homeserver %q: %w", cfg.TeamName, homeserver, err)
	}

	sessions := make(map[string]agentSession)
	for agentName, agentCfg := range matrixCfg.Agents {
		token := os.Getenv(agentCfg.AccessTokenEnv)
		if token == "" {
			log.Printf("[matrix] warning: %s empty for agent %s — skipping", agentCfg.AccessTokenEnv, agentName)
			continue
		}
		userID := id.NewUserID(agentName, domain)
		client, err := mautrix.NewClient(homeserver, userID, token)
		if err != nil {
			return nil, fmt.Errorf("matrix client for %s: %w", agentName, err)
		}
		sessions[agentName] = agentSession{
			client: client,
			roomID: id.RoomID(agentCfg.RoomID),
		}
	}

	// Notification client (optional — log and skip if not configured)
	var notifyClient *mautrix.Client
	var notifyRoom id.RoomID
	if matrixCfg.NotifyTokenEnv != "" && matrixCfg.NotifyRoom != "" {
		token := os.Getenv(matrixCfg.NotifyTokenEnv)
		if token != "" {
			userID := id.NewUserID("notify", domain)
			nc, err := mautrix.NewClient(homeserver, userID, token)
			if err != nil {
				log.Printf("[matrix] warning: notification client failed: %v", err)
			} else {
				notifyClient = nc
				notifyRoom = id.RoomID(matrixCfg.NotifyRoom)
			}
		}
	}

	return &MatrixFrontend{
		cfg:          cfg,
		sessions:     sessions,
		notifyClient: notifyClient,
		notifyRoom:   notifyRoom,
		lastEventID:  make(map[string]id.EventID),
	}, nil
}

// Start begins polling/syncing for inbound messages for each configured agent.
// NOTE: Do NOT start notification client sync in Phase 2 — no event handlers are registered.
// Phase 3 adds notification room command handlers and starts the sync goroutine then.
func (f *MatrixFrontend) Start(ctx context.Context) error {
	ctx, f.cancel = context.WithCancel(ctx)

	for agentName, sess := range f.sessions {
		name := agentName
		s := sess

		syncer := s.client.Syncer.(*mautrix.DefaultSyncer)

		// Skip all events in the initial sync batch (since="") to prevent
		// replaying old messages on daemon restart.
		// DontProcessOldEvents returns false when since="", causing ProcessResponse
		// to return immediately before dispatching any events for that batch.
		syncer.OnSync(s.client.DontProcessOldEvents)

		// Filter: only receive m.room.message events (no presence, typing, read receipts).
		// FilterJSON is set on DefaultSyncer, NOT on Client.
		syncer.FilterJSON = &mautrix.Filter{
			Room: &mautrix.RoomFilter{
				Timeline: &mautrix.FilterPart{
					Types: []event.Type{event.EventMessage},
				},
			},
		}

		syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
			// Skip own messages.
			if evt.Sender == s.client.UserID {
				return
			}
			msg := evt.Content.AsMessage()
			if msg == nil || msg.Body == "" {
				return
			}

			// Track for future reactions (Phase 4).
			f.lastEventMu.Lock()
			f.lastEventID[name] = evt.ID
			f.lastEventMu.Unlock()

			// Persist inbound message (persistMsg is in daemon package and can't be imported;
			// call MsgSvc.Create directly with a nil guard instead).
			senderName := f.cfg.UserNameFn()
			if f.cfg.MsgSvc != nil {
				if _, err := f.cfg.MsgSvc.Create(ctx, message.CreateParams{
					Sender:    senderName,
					Recipient: name,
					Content:   msg.Body,
					Team:      f.cfg.TeamName,
					Channel:   message.ChannelMatrix,
				}); err != nil {
					log.Printf("[matrix] message persist failed (sender=%s): %v", senderName, err)
				}
			}

			// Format and deliver to agent via tmux.
			formatted := fmt.Sprintf("[matrix from:%s] %s", senderName, msg.Body)
			f.cfg.OnMessage(f.cfg.TeamName, name, formatted)
		})

		go func(c *mautrix.Client, n string) {
			if err := c.SyncWithContext(ctx); err != nil && ctx.Err() == nil {
				log.Printf("[matrix] FATAL: sync stopped for agent %s — no messages will be received until restart: %v", n, err)
			}
		}(s.client, name)
	}

	return nil
}

// Stop gracefully shuts down all sync loops.
func (f *MatrixFrontend) Stop(_ context.Context) error {
	f.stopOnce.Do(func() {
		if f.cancel != nil {
			f.cancel()
		}
	})
	return nil
}

// SendText sends a text message to an agent's Matrix room.
// Long messages are split at natural boundaries to stay within the 65535-byte limit.
func (f *MatrixFrontend) SendText(ctx context.Context, agentName, text string) error {
	sess, ok := f.sessions[agentName]
	if !ok {
		return fmt.Errorf("no Matrix session for agent %s", agentName)
	}
	for _, chunk := range splitMatrixMessage(text) {
		if _, err := sess.client.SendText(ctx, sess.roomID, chunk); err != nil {
			return fmt.Errorf("matrix send to %s: %w", agentName, err)
		}
	}
	return nil
}

// SendVoice is a no-op stub — Phase 4 will implement voice message uploads.
func (f *MatrixFrontend) SendVoice(_ context.Context, _ string, _ []byte) error {
	return nil // silent drop — Phase 4
}

// SendNotification sends a system notification to the configured notification room.
// If no notification client is configured, logs a warning and returns nil (not an error).
func (f *MatrixFrontend) SendNotification(ctx context.Context, text string) error {
	if f.notifyClient == nil {
		log.Printf("[matrix] no notification client configured for team %s — dropping notification", f.cfg.TeamName)
		return nil // not an error — acceptable in Phase 2
	}
	if _, err := f.notifyClient.SendText(ctx, f.notifyRoom, text); err != nil {
		return fmt.Errorf("matrix notification: %w", err)
	}
	return nil
}

// SetReaction is a no-op stub — Phase 4 will implement emoji reactions.
func (f *MatrixFrontend) SetReaction(_ context.Context, _ string, _ string) error {
	return nil // no-op — Phase 4
}

// ClearTracking clears the tracked inbound event ID for an agent.
// Called after the agent responds to prevent stale reactions on old messages.
func (f *MatrixFrontend) ClearTracking(_ context.Context, agentName string) error {
	f.lastEventMu.Lock()
	delete(f.lastEventID, agentName)
	f.lastEventMu.Unlock()
	return nil
}

// AskHuman immediately returns skipped — MUST NOT block as the daemon HTTP handler calls it
// synchronously. Phase 3 will implement interactive ask-human via Matrix.
func (f *MatrixFrontend) AskHuman(_ context.Context, _ string, _ string, _ []string) (string, bool, error) {
	return "", true, nil // immediately skipped — MUST NOT block — Phase 3
}

// RegisterCommands is a no-op — Matrix has no native /setMyCommands equivalent.
func (f *MatrixFrontend) RegisterCommands(_ []Command) error {
	return nil
}

const maxMatrixMessageBytes = 65535

// splitMatrixMessage splits text into chunks that fit within Matrix's 65535-byte limit.
// Splits prefer paragraph breaks, then line breaks, then word breaks.
func splitMatrixMessage(text string) []string {
	if len(text) <= maxMatrixMessageBytes {
		return []string{text}
	}
	var parts []string
	for len(text) > 0 {
		if len(text) <= maxMatrixMessageBytes {
			parts = append(parts, text)
			break
		}
		chunk := text[:maxMatrixMessageBytes]
		cutAt := maxMatrixMessageBytes
		if i := strings.LastIndex(chunk, "\n\n"); i > 0 {
			cutAt = i
		} else if i := strings.LastIndex(chunk, "\n"); i > 0 {
			cutAt = i
		} else if i := strings.LastIndex(chunk, " "); i > 0 {
			cutAt = i
		}
		part := strings.TrimRight(text[:cutAt], " \n")
		if part != "" {
			parts = append(parts, part)
		}
		text = strings.TrimLeft(text[cutAt:], " \n")
	}
	return parts
}

// extractDomain extracts the host portion from a homeserver URL.
// Returns an error if the URL is malformed or has no host (which would produce invalid Matrix user IDs).
// e.g. "https://matrix.example.com" → "matrix.example.com"
func extractDomain(homeserverURL string) (string, error) {
	u, err := url.Parse(homeserverURL)
	if err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("no host in URL (missing scheme?)")
	}
	return u.Host, nil
}

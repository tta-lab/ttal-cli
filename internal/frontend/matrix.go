package frontend

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

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

// MatrixFrontend implements Frontend using the Matrix protocol via mautrix-go.
type MatrixFrontend struct {
	cfg          MatrixConfig
	clients      map[string]*mautrix.Client // agentName → client
	roomIDs      map[string]id.RoomID       // agentName → room
	notifyClient *mautrix.Client
	notifyRoom   id.RoomID
	cancel       context.CancelFunc
	stopOnce     sync.Once

	// Track last event ID per agent for future reaction support (Phase 4).
	lastEventMu sync.RWMutex
	lastEventID map[string]id.EventID // agentName → last inbound event ID
}

// NewMatrix constructs a MatrixFrontend from the given config.
// Returns an error if the team's [matrix] config block is missing.
// Agents whose env-var tokens are unset are skipped with a warning (partial setup is OK).
func NewMatrix(cfg MatrixConfig) (*MatrixFrontend, error) {
	team, ok := cfg.MCfg.Teams[cfg.TeamName]
	if !ok {
		return nil, fmt.Errorf("team %q not found in config", cfg.TeamName)
	}
	matrixCfg := team.Matrix
	if matrixCfg == nil {
		return nil, fmt.Errorf("team %q has frontend=matrix but no [teams.%s.matrix] config", cfg.TeamName, cfg.TeamName)
	}

	homeserver := matrixCfg.Homeserver
	domain := extractDomain(homeserver)

	clients := make(map[string]*mautrix.Client)
	roomIDs := make(map[string]id.RoomID)

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
		clients[agentName] = client
		roomIDs[agentName] = id.RoomID(agentCfg.RoomID)
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
		clients:      clients,
		roomIDs:      roomIDs,
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

	for agentName, client := range f.clients {
		name := agentName // capture loop variable
		cli := client     // capture loop variable

		// Skip events until initial sync completes (prevents replaying old messages).
		// mautrix-go's DefaultSyncer processes OnEventType handlers BEFORE OnSync callbacks,
		// so events in the initial batch correctly see ready=false and are skipped.
		var ready atomic.Bool

		syncer := cli.Syncer.(*mautrix.DefaultSyncer)

		// Mark ready after first sync batch completes.
		syncer.OnSync(func(_ context.Context, _ *mautrix.RespSync, _ string) bool {
			if !ready.Load() {
				ready.Store(true)
				log.Printf("[matrix] initial sync complete for %s, now processing events", name)
			}
			return true // continue syncing
		})

		// Filter: only receive m.room.message events (no presence, typing, read receipts).
		// FilterJSON is set on DefaultSyncer, NOT on Client.
		syncer.FilterJSON = &mautrix.Filter{
			Room: &mautrix.RoomFilter{
				Timeline: &mautrix.FilterPart{
					Types: []event.Type{event.EventMessage},
				},
			},
		}

		syncer.OnEventType(event.EventMessage, func(_ context.Context, evt *event.Event) {
			// Skip events from initial sync (old messages).
			if !ready.Load() {
				return
			}
			// Skip own messages.
			if evt.Sender == cli.UserID {
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
				if _, err := f.cfg.MsgSvc.Create(context.Background(), message.CreateParams{
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
				log.Printf("[matrix] sync stopped for %s: %v", n, err)
			}
		}(cli, name)
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
	client, ok := f.clients[agentName]
	if !ok {
		return fmt.Errorf("no Matrix client for agent %s", agentName)
	}
	roomID, ok := f.roomIDs[agentName]
	if !ok {
		return fmt.Errorf("no room ID for agent %s", agentName)
	}
	for _, chunk := range splitMatrixMessage(text) {
		if _, err := client.SendText(ctx, roomID, chunk); err != nil {
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
// e.g. "https://matrix.example.com" → "matrix.example.com"
func extractDomain(homeserverURL string) string {
	u, err := url.Parse(homeserverURL)
	if err != nil || u.Host == "" {
		return homeserverURL
	}
	return u.Host
}

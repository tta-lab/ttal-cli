package frontend

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/message"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/voice"
)

// MatrixConfig holds construction parameters for MatrixFrontend.
type MatrixConfig struct {
	TeamName   string
	MCfg       *config.DaemonConfig
	OnMessage  InboundHandler
	MsgSvc     *message.Service
	UserNameFn func() string // returns human display name for this team
	GetUsageFn func() string // returns formatted usage string, or "" if unavailable
	RestartFn  func() error  // triggers daemon restart (launchctl kickstart -k)
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

	mas         *matrixAskStore // ask-human state
	allCommands []Command       // stored by RegisterCommands for /help and skill dispatch
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
		mas:          newMatrixAskStore(),
	}, nil
}

// Start begins polling/syncing for inbound messages for each configured agent
// and starts the notification client sync with command handlers.
func (f *MatrixFrontend) Start(ctx context.Context) error {
	ctx, f.cancel = context.WithCancel(ctx)
	for agentName, sess := range f.sessions {
		f.startAgentSync(ctx, agentName, sess)
	}
	f.startNotifSync(ctx)
	return nil
}

// startAgentSync sets up the sync loop for one agent session.
func (f *MatrixFrontend) startAgentSync(ctx context.Context, agentName string, sess agentSession) {
	syncer := sess.client.Syncer.(*mautrix.DefaultSyncer)

	// Skip all events in the initial sync batch (since="") to prevent
	// replaying old messages on daemon restart.
	syncer.OnSync(sess.client.DontProcessOldEvents)

	// Filter: only receive m.room.message events (no presence, typing, read receipts).
	syncer.FilterJSON = &mautrix.Filter{
		Room: &mautrix.RoomFilter{
			Timeline: &mautrix.FilterPart{
				Types: []event.Type{event.EventMessage},
			},
		},
	}

	syncer.OnEventType(event.EventMessage, func(ctx context.Context, evt *event.Event) {
		if evt.Sender == sess.client.UserID {
			return
		}
		msg := evt.Content.AsMessage()
		if msg == nil || msg.Body == "" {
			return
		}

		// Track for future reactions (Phase 4).
		f.lastEventMu.Lock()
		f.lastEventID[agentName] = evt.ID
		f.lastEventMu.Unlock()

		body := strings.TrimSpace(msg.Body)

		// 1. Check if this is an answer to a pending ask-human question.
		if f.interceptMatrixAskAnswer(sess.roomID, body) {
			return // consumed as ask-human answer
		}

		// 2. Check if this is a /command.
		if strings.HasPrefix(body, "/") {
			f.handleMatrixCommand(agentName, body, sess.client, sess.roomID)
			return
		}

		// 3. Check if this is a voice/audio message — transcribe it.
		if msg.MsgType == event.MsgAudio {
			f.handleMatrixVoice(ctx, agentName, msg, sess.client)
			return
		}

		// 4. Regular message — persist and deliver.
		f.deliverInboundMessage(ctx, agentName, msg.Body)
	})

	go func() {
		if err := sess.client.SyncWithContext(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[matrix] FATAL: sync stopped for agent %s — restart required: %v", agentName, err)
		}
	}()
}

// deliverInboundMessage persists and forwards a regular inbound message to the agent.
func (f *MatrixFrontend) deliverInboundMessage(ctx context.Context, agentName, body string) {
	senderName := f.cfg.UserNameFn()
	if f.cfg.MsgSvc != nil {
		if _, err := f.cfg.MsgSvc.Create(ctx, message.CreateParams{
			Sender:    senderName,
			Recipient: agentName,
			Content:   body,
			Team:      f.cfg.TeamName,
			Channel:   message.ChannelMatrix,
		}); err != nil {
			log.Printf("[matrix] message persist failed (sender=%s): %v", senderName, err)
		}
	}
	formatted := fmt.Sprintf("[matrix from:%s] %s", senderName, body)
	f.cfg.OnMessage(f.cfg.TeamName, agentName, formatted)
}

// startNotifSync sets up the notification client sync loop with command handlers.
func (f *MatrixFrontend) startNotifSync(ctx context.Context) {
	if f.notifyClient == nil {
		return
	}
	notifSyncer := f.notifyClient.Syncer.(*mautrix.DefaultSyncer)

	notifSyncer.OnSync(f.notifyClient.DontProcessOldEvents)

	notifSyncer.FilterJSON = &mautrix.Filter{
		Room: &mautrix.RoomFilter{
			Timeline: &mautrix.FilterPart{
				Types: []event.Type{event.EventMessage},
			},
		},
	}

	notifSyncer.OnEventType(event.EventMessage, func(_ context.Context, evt *event.Event) {
		if evt.Sender == f.notifyClient.UserID {
			return
		}
		msg := evt.Content.AsMessage()
		if msg == nil || msg.Body == "" {
			return
		}
		body := strings.TrimSpace(msg.Body)
		if strings.HasPrefix(body, "/") {
			f.handleNotifCommand(body)
		}
	})

	go func() {
		if err := f.notifyClient.SyncWithContext(ctx); err != nil && ctx.Err() == nil {
			log.Printf("[matrix] notification sync stopped: %v", err)
		}
	}()
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

// SetReaction sends an emoji reaction on the last tracked inbound message for an agent.
// Matrix reactions are additive (each call adds a new reaction, unlike Telegram which replaces).
func (f *MatrixFrontend) SetReaction(ctx context.Context, agentName string, emoji string) error {
	sess, ok := f.sessions[agentName]
	if !ok {
		return nil // no session — silently skip (same as Telegram)
	}

	f.lastEventMu.RLock()
	evtID, ok := f.lastEventID[agentName]
	f.lastEventMu.RUnlock()
	if !ok {
		return nil // no tracked message — silently skip
	}

	content := &event.ReactionEventContent{
		RelatesTo: event.RelatesTo{
			Type:    event.RelAnnotation,
			EventID: evtID,
			Key:     emoji,
		},
	}
	_, err := sess.client.SendMessageEvent(ctx, sess.roomID, event.EventReaction, content)
	if err != nil {
		return fmt.Errorf("matrix reaction for %s: %w", agentName, err)
	}
	return nil
}

// handleMatrixVoice downloads and transcribes an inbound Matrix voice/audio message,
// then delivers the transcription to the agent as a regular message.
func (f *MatrixFrontend) handleMatrixVoice(ctx context.Context, agentName string, msg *event.MessageEventContent, client *mautrix.Client) {
	if msg.URL == "" {
		log.Printf("[matrix] voice message from %s has no URL — skipping", agentName)
		return
	}

	mxcURL, err := msg.URL.Parse()
	if err != nil {
		log.Printf("[matrix] invalid mxc URL for voice from %s: %v", agentName, err)
		return
	}

	audioData, err := client.DownloadBytes(ctx, mxcURL)
	if err != nil {
		log.Printf("[matrix] voice download failed for %s: %v", agentName, err)
		return
	}

	// Determine filename from message or default to voice.ogg
	filename := "voice.ogg"
	if msg.FileName != "" {
		filename = msg.FileName
	} else if msg.Body != "" && msg.Body != "Voice message" {
		filename = msg.Body
	}

	transcription, err := voice.Transcribe(audioData, filename)
	if err != nil {
		log.Printf("[matrix] voice transcription failed for %s: %v", agentName, err)
		// Notify the human about the failure
		sess, ok := f.sessions[agentName]
		if ok {
			tctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_, _ = sess.client.SendText(tctx, sess.roomID, "Voice transcription failed — check daemon logs for details")
		}
		return
	}

	senderName := f.cfg.UserNameFn()
	rawText := "[🎤 voice] " + transcription

	if f.cfg.MsgSvc != nil {
		if _, err := f.cfg.MsgSvc.Create(ctx, message.CreateParams{
			Sender:    senderName,
			Recipient: agentName,
			Content:   rawText,
			Team:      f.cfg.TeamName,
			Channel:   message.ChannelMatrix,
		}); err != nil {
			log.Printf("[matrix] voice persist failed: %v", err)
		}
	}

	formatted := fmt.Sprintf("[matrix from:%s] %s", senderName, rawText)
	f.cfg.OnMessage(f.cfg.TeamName, agentName, formatted)
}

// ClearTracking clears the tracked inbound event ID for an agent.
// Called after the agent responds to prevent stale reactions on old messages.
func (f *MatrixFrontend) ClearTracking(_ context.Context, agentName string) error {
	f.lastEventMu.Lock()
	delete(f.lastEventID, agentName)
	f.lastEventMu.Unlock()
	return nil
}

// RegisterCommands stores bot commands for /help and skill dispatch.
// Matrix has no native /setMyCommands equivalent, so we only store locally.
func (f *MatrixFrontend) RegisterCommands(commands []Command) error {
	f.allCommands = commands
	return nil
}

// parseMatrixCommand splits a /command[@bot] text into (cmd, args).
func parseMatrixCommand(text string) (string, []string) {
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", nil
	}
	cmd := strings.TrimPrefix(parts[0], "/")
	cmd, _, _ = strings.Cut(cmd, "@") // strip optional @botname suffix
	return cmd, parts[1:]
}

// makeMatrixReplyFn returns a reply function that sends text to the given room.
func makeMatrixReplyFn(client *mautrix.Client, roomID id.RoomID, logTag string) func(string) {
	return func(reply string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := client.SendText(ctx, roomID, reply); err != nil {
			log.Printf("[matrix] %s: reply failed: %v", logTag, err)
		}
	}
}

// handleMatrixCommand parses and dispatches a /command message from an agent room.
func (f *MatrixFrontend) handleMatrixCommand(
	agentName, text string, client *mautrix.Client, roomID id.RoomID,
) {
	cmd, args := parseMatrixCommand(text)
	if cmd == "" {
		return
	}

	replyFn := makeMatrixReplyFn(client, roomID, "agent="+agentName)

	switch cmd {
	case "status":
		f.handleMatrixStatusCommand(f.cfg.TeamName, replyFn, args)
	case "help":
		f.handleMatrixHelpCommand(replyFn)
	case "usage":
		f.handleMatrixUsageCommand(replyFn)
	case "new":
		fullCmd := "/" + cmd + joinArgs(args, " ")
		sendKeysToAgentWithReply(f.cfg.TeamName, agentName, fullCmd, "Sent /new — starting fresh conversation", replyFn)
	case "compact":
		fullCmd := "/" + cmd + joinArgs(args, " ")
		sendKeysToAgentWithReply(f.cfg.TeamName, agentName, fullCmd, "Sent /compact — compacting conversation", replyFn)
	case "wait":
		sendEscToAgentWithReply(f.cfg.TeamName, agentName, replyFn)
	default:
		origName := f.resolveSkillCommand(cmd)
		if origName != "" {
			fullCmd := buildSkillCommand(origName, text)
			sendKeysToAgentWithReply(f.cfg.TeamName, agentName, fullCmd, "", replyFn)
		}
		// Unknown commands are silently ignored (same as Telegram default handler)
	}
}

func (f *MatrixFrontend) handleMatrixStatusCommand(teamName string, replyFn func(string), args []string) {
	var agents []status.AgentStatus
	if len(args) > 0 {
		s, err := status.ReadAgent(teamName, args[0])
		if err != nil {
			replyFn("Error: " + err.Error())
			return
		}
		if s == nil {
			replyFn(args[0] + ": no status data")
			return
		}
		agents = []status.AgentStatus{*s}
	} else {
		all, err := status.ReadAll(teamName)
		if err != nil {
			replyFn("Error reading status: " + err.Error())
			return
		}
		agents = all
	}
	if len(agents) == 0 {
		replyFn("No agent status data available")
		return
	}
	var sb strings.Builder
	for _, a := range agents {
		staleMarker := ""
		if a.IsStale(5 * time.Minute) {
			staleMarker = " (stale)"
		}
		fmt.Fprintf(&sb, "%s: %.0f%% ctx | %s%s\n", a.Agent, a.ContextUsedPct, a.ModelName, staleMarker)
	}
	replyFn(sb.String())
}

func (f *MatrixFrontend) handleMatrixHelpCommand(replyFn func(string)) {
	var sb strings.Builder
	sb.WriteString("Available commands:\n")
	for _, cmd := range f.allCommands {
		fmt.Fprintf(&sb, "/%s — %s\n", cmd.Name, cmd.Description)
	}
	replyFn(sb.String())
}

func (f *MatrixFrontend) handleMatrixUsageCommand(replyFn func(string)) {
	if f.cfg.GetUsageFn == nil {
		replyFn("Usage data not available")
		return
	}
	msg := f.cfg.GetUsageFn()
	if msg == "" {
		replyFn("Usage data not yet available — daemon is still fetching")
		return
	}
	replyFn(msg)
}

func (f *MatrixFrontend) resolveSkillCommand(cmd string) string {
	for _, c := range f.allCommands {
		if c.Name == cmd {
			if c.OriginalName != "" {
				return c.OriginalName
			}
			return c.Name
		}
	}
	return ""
}

// sendKeysToAgentWithReply sends tmux keys to an agent and optionally replies with a confirmation.
func sendKeysToAgentWithReply(teamName, agentName, keys, confirmMsg string, replyFn func(string)) {
	session := config.AgentSessionName(teamName, agentName)
	if err := tmux.SendKeys(session, agentName, keys); err != nil {
		replyFn("Error: " + err.Error())
		return
	}
	if confirmMsg != "" {
		replyFn(confirmMsg)
	}
}

// sendEscToAgentWithReply sends Escape to an agent's tmux session and replies with confirmation.
func sendEscToAgentWithReply(teamName, agentName string, replyFn func(string)) {
	session := config.AgentSessionName(teamName, agentName)
	if err := tmux.SendRawKey(session, agentName, "Escape"); err != nil {
		replyFn("Error: " + err.Error())
		return
	}
	replyFn("Sent Escape — interrupting agent")
}

// handleNotifCommand parses and dispatches a /command message from the notification room.
func (f *MatrixFrontend) handleNotifCommand(text string) {
	cmd, args := parseMatrixCommand(text)
	if cmd == "" {
		return
	}

	replyFn := makeMatrixReplyFn(f.notifyClient, f.notifyRoom, "notif")

	switch cmd {
	case "status":
		f.handleMatrixStatusCommand(f.cfg.TeamName, replyFn, args)
	case "usage":
		f.handleMatrixUsageCommand(replyFn)
	case "restart":
		if f.cfg.RestartFn == nil {
			replyFn("⚠️ Restart not configured")
			return
		}
		if err := f.cfg.RestartFn(); err != nil {
			log.Printf("[matrix] restart failed: %v", err)
			replyFn("❌ Restart failed: " + err.Error())
			return
		}
		replyFn("🔄 Daemon restarting...")
	case "help":
		var sb strings.Builder
		sb.WriteString("Notification commands:\n")
		for _, c := range matrixNotifCommands {
			fmt.Fprintf(&sb, "/%s — %s\n", c.Name, c.Description)
		}
		replyFn(sb.String())
	}
}

var matrixNotifCommands = []Command{
	{Name: "status", Description: "Show all agents' context usage and stats"},
	{Name: "usage", Description: "Show Claude API 5hr/weekly rate limit consumption"},
	{Name: "restart", Description: "Restart the daemon (launchctl kickstart -k)"},
	{Name: "help", Description: "List available commands"},
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

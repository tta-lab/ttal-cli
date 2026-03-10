package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"time"

	"encoding/json"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/ent/message"

	_ "modernc.org/sqlite" // SQLite driver — registers "sqlite" dialect used by entsql.Open
)

const socketTimeout = 5 * time.Second

// Contact summarises a conversation partner shown in the sidebar.
type Contact struct {
	Name          string    `json:"name"`
	LastMessageAt time.Time `json:"lastMessageAt"`
}

// SendRequest mirrors daemon.SendRequest — defined locally to avoid pulling in
// the full daemon package (which brings in CGO-incompatible deps via watcher).
//
// Routing semantics (source: internal/daemon/socket.go):
//
//	From only:    agent → human via Telegram
//	To only:      system/hook → agent via tmux  ← ChatService.SendMessage uses this
//	From + To:    agent → agent via tmux with attribution
//
// ChatService.SendMessage uses To-only so the daemon routes via handleTo →
// deliverToAgent, the same path Telegram messages use. Do NOT set From here —
// it would route to handleAgentToAgent which requires a registered agent named
// after the sender and would fail for the human user.
type SendRequest struct {
	From    string `json:"from,omitempty"`
	To      string `json:"to,omitempty"`
	Team    string `json:"team,omitempty"`
	Message string `json:"message"`
}

// ChatService is the Go backend exposed to the Svelte frontend via Wails.
// Read operations query SQLite directly; write operations go through the
// daemon unix socket so delivery semantics are consistent with CLI sends.
type ChatService struct {
	db       *ent.Client          // SQLite client (read + local writes like reactions)
	sockPath string               // daemon unix socket for message delivery
	userName string               // human identity (e.g. "neil")
	mcfg     *config.DaemonConfig // for resolving agent workspaces
}

// NewChatService opens the Ent client and prepares the ChatService.
// sockPath defaults to ~/.ttal/daemon.sock if empty.
func NewChatService(dbPath, sockPath string, mcfg *config.DaemonConfig) (*ChatService, error) {
	// Use the same open pattern as the daemon: modernc/sqlite + WAL mode.
	dsn := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	drv, err := entsql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB("sqlite3", drv.DB())))

	userName := ""
	if mcfg != nil && mcfg.Global != nil {
		userName = mcfg.Global.UserName()
	}
	if userName == "" {
		userName = os.Getenv("USER")
	}
	if userName == "" {
		return nil, fmt.Errorf("could not determine user name — set [user] name in config.toml or $USER env")
	}

	if sockPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("home dir: %w", err)
		}
		sockPath = filepath.Join(home, ".ttal", "daemon.sock")
	}

	return &ChatService{
		db:       client,
		sockPath: sockPath,
		userName: userName,
		mcfg:     mcfg,
	}, nil
}

// Close releases the database connection.
func (s *ChatService) Close() error {
	return s.db.Close()
}

// GetUserName returns the configured human identity.
func (s *ChatService) GetUserName() string {
	return s.userName
}

// GetMessages returns paginated messages between userA and userB.
func (s *ChatService) GetMessages(userA, userB string, limit, offset int) ([]*ent.Message, error) {
	ctx := context.Background()
	msgs, err := s.db.Message.Query().
		Where(
			message.Or(
				message.And(message.SenderEQ(userA), message.RecipientEQ(userB)),
				message.And(message.SenderEQ(userB), message.RecipientEQ(userA)),
			),
		).
		Order(ent.Asc(message.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	return msgs, nil
}

// GetContacts returns contacts sorted by most-recent message descending.
// Each contact is an agent that has exchanged messages with the human user.
func (s *ChatService) GetContacts() ([]Contact, error) {
	ctx := context.Background()
	// Fetch recent messages with a reasonable cap to bound memory use.
	const maxRows = 5000
	msgs, err := s.db.Message.Query().
		Where(
			message.Or(
				message.SenderEQ(s.userName),
				message.RecipientEQ(s.userName),
			),
		).
		Order(ent.Desc(message.FieldCreatedAt)).
		Limit(maxRows).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}

	// Keep only the most-recent timestamp per partner.
	seen := map[string]time.Time{}
	for _, m := range msgs {
		partner := m.Recipient
		if m.Sender != s.userName {
			partner = m.Sender
		}
		if partner == s.userName {
			continue
		}
		if _, ok := seen[partner]; !ok {
			seen[partner] = m.CreatedAt
		}
	}

	contacts := make([]Contact, 0, len(seen))
	for name, ts := range seen {
		contacts = append(contacts, Contact{Name: name, LastMessageAt: ts})
	}
	sort.Slice(contacts, func(i, j int) bool {
		return contacts[i].LastMessageAt.After(contacts[j].LastMessageAt)
	})
	return contacts, nil
}

// GetAgentFeedMessages returns messages that are agent-to-agent (neither sender
// nor recipient is the human user) — useful for a broadcast/feed view.
func (s *ChatService) GetAgentFeedMessages(limit, offset int) ([]*ent.Message, error) {
	ctx := context.Background()
	msgs, err := s.db.Message.Query().
		Where(
			message.And(
				message.SenderNEQ(s.userName),
				message.RecipientNEQ(s.userName),
			),
		).
		Order(ent.Desc(message.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query feed: %w", err)
	}
	return msgs, nil
}

// GetAvatar reads the avatar image for agentName from its workspace .assets directory.
// Returns raw bytes (PNG/JPEG). The frontend converts these to a blob URL.
func (s *ChatService) GetAvatar(agentName string) ([]byte, error) {
	if s.mcfg == nil {
		return nil, fmt.Errorf("no daemon config")
	}
	for _, team := range s.mcfg.Teams {
		if team.TeamPath == "" {
			continue
		}
		info, err := agentfs.Get(team.TeamPath, agentName)
		if err != nil {
			// Agent not in this team — try the next one.
			continue
		}
		for _, ext := range []string{".png", ".jpg", ".jpeg"} {
			p := filepath.Join(info.Path, "assets", "avatar"+ext)
			data, err := os.ReadFile(p)
			if err == nil {
				return data, nil
			}
			if !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("read avatar %s: %w", p, err)
			}
		}
	}
	return nil, fmt.Errorf("avatar not found for agent %q", agentName)
}

// TeamInfo groups a team name with its agents for the sidebar.
type TeamInfo struct {
	Name   string         `json:"name"`
	Agents []AgentSummary `json:"agents"`
}

// AgentSummary holds the minimal agent info needed by the sidebar.
type AgentSummary struct {
	Name        string `json:"name"`
	Emoji       string `json:"emoji"`
	Description string `json:"description"`
}

// GetTeams returns all teams and their agents from daemon config.
func (s *ChatService) GetTeams() ([]TeamInfo, error) {
	if s.mcfg == nil {
		return nil, fmt.Errorf("no daemon config")
	}
	var teams []TeamInfo
	for teamName, team := range s.mcfg.Teams {
		if team.TeamPath == "" {
			continue
		}
		agents, err := agentfs.Discover(team.TeamPath)
		if err != nil {
			log.Printf("[gui] GetTeams: agentfs.Discover(%q) failed for team %q: %v", team.TeamPath, teamName, err)
			continue
		}
		ti := TeamInfo{Name: teamName}
		for _, a := range agents {
			ti.Agents = append(ti.Agents, AgentSummary{
				Name:        a.Name,
				Emoji:       a.Emoji,
				Description: a.Description,
			})
		}
		sort.Slice(ti.Agents, func(i, j int) bool {
			return ti.Agents[i].Name < ti.Agents[j].Name
		})
		teams = append(teams, ti)
	}
	sort.Slice(teams, func(i, j int) bool {
		return teams[i].Name < teams[j].Name
	})
	return teams, nil
}

// dialDaemon opens a connection to the daemon unix socket.
func (s *ChatService) dialDaemon() (net.Conn, error) {
	conn, err := net.DialTimeout("unix", s.sockPath, socketTimeout)
	if err != nil {
		return nil, fmt.Errorf("daemon not running: %w", err)
	}
	return conn, nil
}

// SendMessage delivers a message to recipient through the daemon socket.
// Uses To-only routing so the daemon handles delivery via handleTo.
func (s *ChatService) SendMessage(recipient, content string) error {
	if content == "" {
		return fmt.Errorf("message content must not be empty")
	}
	req := SendRequest{To: recipient, Message: content}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	conn, err := s.dialDaemon()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(socketTimeout)); err != nil {
		return fmt.Errorf("set deadline: %w", err)
	}

	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

// AddReaction writes a reaction directly to the local SQLite database.
// Reactions are display-only and do not require daemon routing.
func (s *ChatService) AddReaction(messageID, emoji string) error {
	msgUUID, err := uuid.Parse(messageID)
	if err != nil {
		return fmt.Errorf("invalid message id: %w", err)
	}
	ctx := context.Background()
	_, err = s.db.Reaction.Create().
		SetID(uuid.New()).
		SetEmoji(emoji).
		SetFromAgent(s.userName).
		SetMessageID(msgUUID).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("add reaction: %w", err)
	}
	return nil
}

// IsDaemonRunning returns true if the daemon socket is reachable.
func (s *ChatService) IsDaemonRunning() bool {
	conn, err := s.dialDaemon()
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

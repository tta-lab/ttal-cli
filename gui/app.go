package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"encoding/json"

	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	"github.com/tta-lab/ttal-cli/internal/ent/message"
)

const socketTimeout = 5 * time.Second

// Contact summarises a conversation partner shown in the sidebar.
type Contact struct {
	Name          string    `json:"name"`
	LastMessageAt time.Time `json:"lastMessageAt"`
}

// SendRequest mirrors daemon.SendRequest — defined locally to avoid pulling in
// the full daemon package (which brings in CGO-incompatible deps via watcher).
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
	client, err := ent.Open("sqlite3", dbPath+"?_fk=1")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	userName := ""
	if mcfg != nil && mcfg.Global != nil {
		userName = mcfg.Global.UserName()
	}
	if userName == "" {
		userName = os.Getenv("USER")
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

// GetContacts returns a list of contacts derived from message history.
// Each contact is an agent that has exchanged messages with the human user.
func (s *ChatService) GetContacts() ([]Contact, error) {
	ctx := context.Background()
	msgs, err := s.db.Message.Query().
		Where(
			message.Or(
				message.SenderEQ(s.userName),
				message.RecipientEQ(s.userName),
			),
		).
		Order(ent.Desc(message.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}

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
			continue
		}
		for _, ext := range []string{".png", ".jpg", ".jpeg"} {
			p := filepath.Join(info.Path, ".assets", "avatar"+ext)
			data, err := os.ReadFile(p)
			if err == nil {
				return data, nil
			}
		}
	}
	return nil, fmt.Errorf("avatar not found for agent %q", agentName)
}

// SendMessage delivers a message to recipient through the daemon socket.
// Uses To-only routing so the daemon handles delivery via handleTo.
func (s *ChatService) SendMessage(recipient, content string) error {
	req := SendRequest{To: recipient, Message: content}
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	conn, err := net.DialTimeout("unix", s.sockPath, socketTimeout)
	if err != nil {
		return fmt.Errorf("daemon not running: %w", err)
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(socketTimeout)) //nolint:errcheck

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
	conn, err := net.DialTimeout("unix", s.sockPath, socketTimeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

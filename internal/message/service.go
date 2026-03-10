// Package message provides CRUD operations for persisting daemon messages.
//
// Plane: manager
package message

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/ent"
	entmessage "github.com/tta-lab/ttal-cli/internal/ent/message"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Channel identifies how a message entered the daemon.
type Channel string

const (
	ChannelTelegram Channel = "telegram"
	ChannelGUI      Channel = "gui"
	ChannelCLI      Channel = "cli"
	ChannelWatcher  Channel = "watcher"
	ChannelAdapter  Channel = "adapter"
)

// Contact is a summary of a conversation partner.
type Contact struct {
	Name          string
	LastMessageAt time.Time
}

// Service wraps the Ent client for message persistence operations.
type Service struct {
	client *ent.Client
}

// NewService creates a new message Service.
func NewService(client *ent.Client) *Service {
	return &Service{client: client}
}

// CreateParams holds parameters for creating a message.
type CreateParams struct {
	Sender    string
	Recipient string
	Content   string
	Team      string
	Channel   Channel
	Runtime   *runtime.Runtime // nil for human messages
}

// Create persists a new message.
func (s *Service) Create(ctx context.Context, p CreateParams) (*ent.Message, error) {
	q := s.client.Message.Create().
		SetSender(p.Sender).
		SetRecipient(p.Recipient).
		SetContent(p.Content).
		SetTeam(p.Team).
		SetChannel(entmessage.Channel(p.Channel))

	if p.Runtime != nil {
		q = q.SetRuntime(string(*p.Runtime))
	}

	return q.Save(ctx)
}

// ListConversation returns messages between two users, newest first.
func (s *Service) ListConversation(ctx context.Context, userA, userB string, limit, offset int) ([]*ent.Message, error) {
	return s.client.Message.Query().
		Where(
			entmessage.Or(
				entmessage.And(
					entmessage.SenderEQ(userA),
					entmessage.RecipientEQ(userB),
				),
				entmessage.And(
					entmessage.SenderEQ(userB),
					entmessage.RecipientEQ(userA),
				),
			),
		).
		Order(ent.Desc(entmessage.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
}

// ListContacts returns the conversation partners of a user, ordered by most recent message.
func (s *Service) ListContacts(ctx context.Context, userID string) ([]Contact, error) {
	msgs, err := s.client.Message.Query().
		Where(
			entmessage.Or(
				entmessage.SenderEQ(userID),
				entmessage.RecipientEQ(userID),
			),
		).
		Order(ent.Desc(entmessage.FieldCreatedAt)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var contacts []Contact
	for _, m := range msgs {
		partner := m.Recipient
		if partner == userID {
			partner = m.Sender
		}
		if !seen[partner] {
			seen[partner] = true
			contacts = append(contacts, Contact{
				Name:          partner,
				LastMessageAt: m.CreatedAt,
			})
		}
	}
	return contacts, nil
}

// ListAgentFeed returns agent-to-agent messages (neither sent nor received by userName), newest first.
func (s *Service) ListAgentFeed(ctx context.Context, userName string, limit, offset int) ([]*ent.Message, error) {
	return s.client.Message.Query().
		Where(
			entmessage.And(
				entmessage.SenderNEQ(userName),
				entmessage.RecipientNEQ(userName),
			),
		).
		Order(ent.Desc(entmessage.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		All(ctx)
}

// AddReaction attaches an emoji reaction to a message.
func (s *Service) AddReaction(ctx context.Context, messageID uuid.UUID, emoji, from string) (*ent.Reaction, error) {
	return s.client.Reaction.Create().
		SetEmoji(emoji).
		SetFromAgent(from).
		SetMessageID(messageID).
		Save(ctx)
}

// AddAttachment attaches a file to a message.
func (s *Service) AddAttachment(ctx context.Context, messageID uuid.UUID, filename, mimeType, path string, size int64) (*ent.Attachment, error) {
	return s.client.Attachment.Create().
		SetFilename(filename).
		SetMimeType(mimeType).
		SetPath(path).
		SetSizeBytes(size).
		SetMessageID(messageID).
		Save(ctx)
}

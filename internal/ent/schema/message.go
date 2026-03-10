package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Message holds the schema definition for the Message entity.
type Message struct {
	ent.Schema
}

// Fields of the Message.
func (Message) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("sender").
			NotEmpty(),
		field.String("recipient").
			NotEmpty().
			Comment("use 'broadcast' for broadcast messages"),
		field.Text("content"),
		field.Enum("message_type").
			Values("text", "system", "notification").
			Default("text"),
		field.String("team").
			NotEmpty(),
		field.Enum("channel").
			Values("telegram", "gui", "cli", "watcher", "adapter"),
		field.String("runtime").
			Optional().
			Validate(func(s string) error {
				if s == "" {
					return nil
				}
				return runtime.Validate(s)
			}).
			Comment("optional — nil for human messages"),
		field.UUID("reply_to_id", uuid.UUID{}).
			Optional().
			Nillable(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Message.
func (Message) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("attachments", Attachment.Type),
		edge.To("reactions", Reaction.Type),
	}
}

// Indexes of the Message.
func (Message) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("sender", "recipient", "created_at"),
		index.Fields("created_at"),
		index.Fields("reply_to_id"),
		index.Fields("team", "created_at"),
	}
}

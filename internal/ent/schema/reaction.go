package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Reaction holds the schema definition for the Reaction entity.
type Reaction struct {
	ent.Schema
}

// Fields of the Reaction.
func (Reaction) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("emoji"),
		field.String("from_agent"),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Edges of the Reaction.
func (Reaction) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("message", Message.Type).
			Ref("reactions").
			Required().
			Unique(),
	}
}

// Indexes of the Reaction.
func (Reaction) Indexes() []ent.Index {
	return []ent.Index{
		// Prevent duplicate reactions (same emoji from same agent on same message).
		index.Fields("emoji", "from_agent").
			Edges("message").
			Unique(),
	}
}

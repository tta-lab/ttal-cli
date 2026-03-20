package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Comment holds the schema definition for the Comment entity.
type Comment struct {
	ent.Schema
}

// Fields of the Comment.
func (Comment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("target").
			NotEmpty().
			Comment("taskwarrior task UUID"),
		field.String("author").
			NotEmpty(),
		field.Text("body"),
		field.Int("round").
			NonNegative(),
		field.String("team").
			NotEmpty(),
		field.Time("created_at").
			Default(time.Now).
			Immutable(),
	}
}

// Indexes of the Comment.
func (Comment) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("target", "created_at"),
		index.Fields("team", "created_at"),
	}
}

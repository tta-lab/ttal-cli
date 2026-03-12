package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// ToolUsage holds the schema definition for the ToolUsage entity.
type ToolUsage struct {
	ent.Schema
}

// Fields of the ToolUsage.
func (ToolUsage) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).Default(uuid.New),
		field.String("agent").NotEmpty(),
		field.String("team").NotEmpty(),
		field.String("command").NotEmpty(),
		field.String("subcommand").NotEmpty(),
		field.String("target").Optional(),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

// Indexes of the ToolUsage.
func (ToolUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("agent", "created_at"),
		index.Fields("agent", "subcommand", "created_at"),
	}
}

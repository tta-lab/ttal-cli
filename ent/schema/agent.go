package schema

import (
	"fmt"
	"strings"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Agent holds the schema definition for the Agent entity.
type Agent struct {
	ent.Schema
}

// Fields of the Agent.
func (Agent) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			Validate(func(s string) error {
				if s != strings.ToLower(s) {
					return fmt.Errorf("agent name must be lowercase")
				}
				return nil
			}).
			Comment("Agent name (unique identifier, lowercase)"),
		field.String("path").
			Optional().
			Comment("Agent workspace path"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("Creation timestamp"),
	}
}

// Edges of the Agent.
func (Agent) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tags", Tag.Type).
			Comment("Agent tags (M2M relation)"),
	}
}

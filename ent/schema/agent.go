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
		field.String("voice").
			Optional().
			Comment("Kokoro TTS voice ID (e.g. af_heart, af_sky)"),
		field.String("emoji").
			Optional().
			Comment("Display emoji (e.g. 🐱, 🦅)"),
		field.String("description").
			Optional().
			Comment("Short role summary (e.g. 'Task orchestration and planning')"),
		field.Enum("model").
			Values("haiku", "sonnet", "opus").
			Default("opus").
			Comment("Claude model tier (haiku, sonnet, opus)"),
		field.Enum("runtime").
			Values("claude-code", "opencode").
			Optional().
			Nillable().
			Comment("Coding agent runtime override. Nil = use team default."),
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

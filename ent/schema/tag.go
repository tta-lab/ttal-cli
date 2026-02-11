package schema

import (
	"fmt"
	"strings"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Tag holds the schema definition for the Tag entity.
type Tag struct {
	ent.Schema
}

// Fields of the Tag.
func (Tag) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Unique().
			NotEmpty().
			Validate(func(s string) error {
				if s != strings.ToLower(s) {
					return fmt.Errorf("tag name must be lowercase")
				}
				return nil
			}).
			Comment("Tag name (lowercase)"),
	}
}

// Edges of the Tag.
func (Tag) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("projects", Project.Type).
			Ref("tags").
			Comment("Projects with this tag"),
		edge.From("agents", Agent.Type).
			Ref("tags").
			Comment("Agents with this tag"),
	}
}

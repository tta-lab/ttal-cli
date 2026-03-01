package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Project holds the schema definition for the Project entity.
type Project struct {
	ent.Schema
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("alias").
			Unique().
			NotEmpty().
			Comment("Project alias (unique identifier)"),
		field.String("name").
			NotEmpty().
			Comment("Project name"),
		field.String("description").
			Optional().
			Comment("Project description"),
		field.String("path").
			Optional().
			Comment("Filesystem path"),
		field.Time("archived_at").
			Optional().
			Nillable().
			Comment("Archived timestamp (NULL = active)"),
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("Creation timestamp"),
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("Last update timestamp"),
	}
}

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return nil
}

// Indexes of the Project.
func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("archived_at"),
	}
}

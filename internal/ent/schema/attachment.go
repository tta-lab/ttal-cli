package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/google/uuid"
)

// Attachment holds the schema definition for the Attachment entity.
type Attachment struct {
	ent.Schema
}

// Fields of the Attachment.
func (Attachment) Fields() []ent.Field {
	return []ent.Field{
		field.UUID("id", uuid.UUID{}).
			Default(uuid.New),
		field.String("filename").
			NotEmpty(),
		field.String("mime_type").
			NotEmpty(),
		field.Int64("size_bytes").
			Min(0),
		field.String("path").
			NotEmpty().
			Comment("relative path: {YYYY-MM}/{message_id}/{filename}"),
		field.String("thumbnail_path").
			Optional().
			Nillable(),
	}
}

// Edges of the Attachment.
func (Attachment) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("message", Message.Type).
			Ref("attachments").
			Required().
			Unique(),
	}
}

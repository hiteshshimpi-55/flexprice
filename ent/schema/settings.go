package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/flexprice/flexprice/ent/schema/mixin"
)

// Settings holds the schema definition for the Settings entity.
type Settings struct {
	ent.Schema
}

// Mixin of the Settings.
func (Settings) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.BaseMixin{},
		mixin.EnvironmentMixin{},
	}
}

// Fields of the Settings.
func (Settings) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").
			SchemaType(map[string]string{
				"postgres": "varchar(50)",
			}).
			Unique().
			Immutable(),
		field.String("key").
			NotEmpty().
			Comment("Key of the setting"),
		field.JSON("value", map[string]string{}).
			Optional().
			SchemaType(map[string]string{
				"postgres": "jsonb",
			}),
	}
}

// Edges of the Settings.
func (Settings) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Indexes of the Settings.
func (Settings) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "environment_id", "key"),
	}
}

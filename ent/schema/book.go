package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// Book holds the schema definition for the Book entity.
type Book struct {
	ent.Schema
}

// Fields of the Book.
func (Book) Fields() []ent.Field {
	return []ent.Field{
		field.String("isbn").Unique(),
		field.String("title"),
		field.Strings("authors"),
		field.String("url"),
	}
}

// Edges of the Book.
func (Book) Edges() []ent.Edge {
	return nil
}

package relation

import "github.com/rapide-go/rapide/database/query"

// BelongsTo represents an Eloquent-style BelongsTo relationship to parent model T.
//
// Example on a Post model:
//
//	func (p Post) User() *relation.BelongsTo[User] {
//	    return relation.BelongsToOf[User](p.UserID)
//	}
type BelongsTo[T any] struct {
	ownerKey string
	foreign  any
}

// BelongsToOf constructs a BelongsTo[T] relation targeting ownerKey (default "id").
func BelongsToOf[T any](foreignValue any, ownerKey ...string) *BelongsTo[T] {
	key := "id"
	if len(ownerKey) > 0 && ownerKey[0] != "" {
		key = ownerKey[0]
	}
	return &BelongsTo[T]{
		ownerKey: key,
		foreign:  foreignValue,
	}
}

// Query returns a fluent query.Builder[T] constrained to the parent record.
func (r *BelongsTo[T]) Query() *query.Builder[T] {
	return query.Query[T]().Where(r.ownerKey, "=", r.foreign)
}

// First fetches the related parent model T.
func (r *BelongsTo[T]) First() (T, error) {
	return r.Query().First()
}

// HasMany represents an Eloquent-style HasMany relationship to child models T.
//
// Example on a User model:
//
//	func (u User) Posts() *relation.HasMany[Post] {
//	    return relation.HasManyOf[Post]("user_id", u.ID)
//	}
type HasMany[T any] struct {
	foreignKey string
	localValue any
}

// HasManyOf constructs a HasMany[T] relation where child.foreignKey = localValue.
func HasManyOf[T any](foreignKey string, localValue any) *HasMany[T] {
	return &HasMany[T]{
		foreignKey: foreignKey,
		localValue: localValue,
	}
}

// Query returns a fluent query.Builder[T] scoped to this parent's children.
func (r *HasMany[T]) Query() *query.Builder[T] {
	return query.Query[T]().Where(r.foreignKey, "=", r.localValue)
}

// Get fetches all related child models []T.
func (r *HasMany[T]) Get() ([]T, error) {
	return r.Query().Get()
}

// Count returns the number of related child records.
func (r *HasMany[T]) Count() (int, error) {
	return r.Query().Count()
}

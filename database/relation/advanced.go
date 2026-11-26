package relation

import (
	"context"
	"fmt"
	"strings"

	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/query"
)

// HasOne represents an Eloquent-style one-to-one relationship to child model T.
type HasOne[T any] struct {
	foreignKey string
	localValue any
}

// HasOneOf constructs a HasOne[T] relation where child.foreignKey = localValue.
func HasOneOf[T any](foreignKey string, localValue any) *HasOne[T] {
	return &HasOne[T]{foreignKey: foreignKey, localValue: localValue}
}

func (r *HasOne[T]) Query() *query.Builder[T] {
	return query.Query[T]().Where(r.foreignKey, "=", r.localValue)
}

func (r *HasOne[T]) First() (T, error) {
	return r.Query().First()
}

// MorphOne represents an Eloquent-style polymorphic one-to-one relationship (`*_type` + `*_id`).
type MorphOne[T any] struct {
	typeCol   string
	idCol     string
	morphType string
	localID   any
}

// MorphOneOf constructs a MorphOne[T] relation (e.g. MorphOneOf[Media]("mediable", "Title", t.ID)).
func MorphOneOf[T any](morphName, morphType string, localID any) *MorphOne[T] {
	return &MorphOne[T]{
		typeCol:   morphName + "_type",
		idCol:     morphName + "_id",
		morphType: morphType,
		localID:   localID,
	}
}

func (r *MorphOne[T]) Query() *query.Builder[T] {
	return query.Query[T]().
		Where(r.typeCol, "=", r.morphType).
		Where(r.idCol, "=", r.localID)
}

func (r *MorphOne[T]) First() (T, error) {
	return r.Query().First()
}

// MorphMany represents an Eloquent-style polymorphic one-to-many relationship (`*_type` + `*_id`).
type MorphMany[T any] struct {
	typeCol   string
	idCol     string
	morphType string
	localID   any
}

// MorphManyOf constructs a MorphMany[T] relation (e.g. MorphManyOf[Comment]("commentable", "Title", t.ID)).
func MorphManyOf[T any](morphName, morphType string, localID any) *MorphMany[T] {
	return &MorphMany[T]{
		typeCol:   morphName + "_type",
		idCol:     morphName + "_id",
		morphType: morphType,
		localID:   localID,
	}
}

func (r *MorphMany[T]) Query() *query.Builder[T] {
	return query.Query[T]().
		Where(r.typeCol, "=", r.morphType).
		Where(r.idCol, "=", r.localID)
}

func (r *MorphMany[T]) Get() ([]T, error)   { return r.Query().Get() }
func (r *MorphMany[T]) Count() (int, error) { return r.Query().Count() }

// BelongsToMany represents an Eloquent-style many-to-many relationship through a pivot table.
type BelongsToMany[T any] struct {
	pivotTable      string
	foreignPivotKey string
	relatedPivotKey string
	relatedKey      string
	localID         any
}

// BelongsToManyOf constructs a BelongsToMany[T] relation over pivotTable.
//
// Example:
//
//	func (t Title) Genres() *relation.BelongsToMany[Genre] {
//	    return relation.BelongsToManyOf[Genre]("genre_title", "title_id", "genre_id", t.ID)
//	}
func BelongsToManyOf[T any](pivotTable, foreignPivotKey, relatedPivotKey string, localID any, relatedKey ...string) *BelongsToMany[T] {
	rk := "id"
	if len(relatedKey) > 0 && relatedKey[0] != "" {
		rk = relatedKey[0]
	}
	return &BelongsToMany[T]{
		pivotTable:      pivotTable,
		foreignPivotKey: foreignPivotKey,
		relatedPivotKey: relatedPivotKey,
		relatedKey:      rk,
		localID:         localID,
	}
}

func (r *BelongsToMany[T]) relatedIDs(ctx context.Context) ([]any, error) {
	for _, id := range []string{r.pivotTable, r.foreignPivotKey, r.relatedPivotKey} {
		if err := dialect.ValidateIdentifier(id); err != nil {
			return nil, err
		}
	}
	d := query.DefaultDialect
	q := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = %s",
		d.Quote(r.relatedPivotKey),
		d.Quote(r.pivotTable),
		d.Quote(r.foreignPivotKey),
		d.Placeholder(1),
	)
	rows, err := query.DB().QueryContext(ctx, q, r.localID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []any
	for rows.Next() {
		var id any
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Get fetches all related models T linked through the pivot table.
func (r *BelongsToMany[T]) Get() ([]T, error) {
	ids, err := r.relatedIDs(context.Background())
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	return query.Query[T]().WhereIn(r.relatedKey, ids).Get()
}

// Count returns the number of linked rows in the pivot table.
func (r *BelongsToMany[T]) Count() (int, error) {
	ids, err := r.relatedIDs(context.Background())
	return len(ids), err
}

// Attach inserts one or more related IDs into the pivot table.
func (r *BelongsToMany[T]) Attach(ids ...any) error {
	for _, id := range []string{r.pivotTable, r.foreignPivotKey, r.relatedPivotKey} {
		if err := dialect.ValidateIdentifier(id); err != nil {
			return err
		}
	}
	d := query.DefaultDialect
	q := fmt.Sprintf(
		"INSERT INTO %s (%s, %s) VALUES (%s, %s)",
		d.Quote(r.pivotTable),
		d.Quote(r.foreignPivotKey),
		d.Quote(r.relatedPivotKey),
		d.Placeholder(1),
		d.Placeholder(2),
	)
	for _, rid := range ids {
		if _, err := query.DB().Exec(q, r.localID, rid); err != nil {
			return err
		}
	}
	return nil
}

// Detach removes the given related IDs from the pivot table (or all links if ids is empty).
func (r *BelongsToMany[T]) Detach(ids ...any) error {
	for _, id := range []string{r.pivotTable, r.foreignPivotKey, r.relatedPivotKey} {
		if err := dialect.ValidateIdentifier(id); err != nil {
			return err
		}
	}
	d := query.DefaultDialect
	if len(ids) == 0 {
		q := fmt.Sprintf("DELETE FROM %s WHERE %s = %s",
			d.Quote(r.pivotTable), d.Quote(r.foreignPivotKey), d.Placeholder(1))
		_, err := query.DB().Exec(q, r.localID)
		return err
	}
	placeholders := make([]string, len(ids))
	args := make([]any, 0, len(ids)+1)
	args = append(args, r.localID)
	for i, rid := range ids {
		placeholders[i] = d.Placeholder(i + 2)
		args = append(args, rid)
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s = %s AND %s IN (%s)",
		d.Quote(r.pivotTable), d.Quote(r.foreignPivotKey), d.Placeholder(1),
		d.Quote(r.relatedPivotKey), strings.Join(placeholders, ", "))
	_, err := query.DB().Exec(q, args...)
	return err
}

// Sync replaces all pivot links for this model with the exact set of ids.
func (r *BelongsToMany[T]) Sync(ids ...any) error {
	if err := r.Detach(); err != nil {
		return err
	}
	return r.Attach(ids...)
}

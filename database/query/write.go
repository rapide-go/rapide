package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/model"
)

// Create inserts a new record, running BeforeCreate hooks, observers, and setting timestamps automatically.
func (b *Builder[T]) Create(item T) (T, error) {
	if b.err != nil {
		return item, b.err
	}
	if hook, ok := any(&item).(model.Creating); ok {
		if err := hook.BeforeCreate(); err != nil {
			return item, err
		}
	}
	obs := getObservers[T]()
	for _, o := range obs {
		if o.Creating != nil {
			if err := o.Creating(&item); err != nil {
				return item, err
			}
		}
	}

	now := time.Now().UTC()
	b.meta.setTimestamp(&item, "created_at", now)
	b.meta.setTimestamp(&item, "updated_at", now)

	cols, vals := b.meta.extract(&item, true)
	quotedCols := make([]string, len(cols))
	placeholders := make([]string, len(cols))
	for i, c := range cols {
		quotedCols[i] = b.dialect.Quote(c)
		placeholders[i] = b.dialect.Placeholder(i + 1)
	}

	q := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		b.dialect.Quote(b.table),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
	res, err := b.exec.ExecContext(b.ctx, q, vals...)
	if err != nil {
		return item, err
	}

	if id, err := res.LastInsertId(); err == nil && id > 0 {
		b.meta.setID(&item, id)
	}
	for _, o := range obs {
		if o.Created != nil {
			o.Created(&item)
		}
	}
	return item, nil
}

// Update updates all rows matching the current WHERE clauses with the given column map.
func (b *Builder[T]) Update(values map[string]any) (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	if len(values) == 0 {
		return 0, nil
	}
	obs := getObservers[T]()
	for _, o := range obs {
		if o.Updating != nil {
			if err := o.Updating(values); err != nil {
				return 0, err
			}
		}
	}

	if b.meta.hasCol("updated_at") {
		if _, exists := values["updated_at"]; !exists {
			values["updated_at"] = time.Now().UTC()
		}
	}

	sets := make([]string, 0, len(values))
	args := make([]any, 0, len(values)+len(b.wheres))
	idx := 1
	for k, v := range values {
		if err := dialect.ValidateIdentifier(k); err != nil {
			return 0, err
		}
		sets = append(sets, fmt.Sprintf("%s = %s", b.dialect.Quote(k), b.dialect.Placeholder(idx)))
		args = append(args, v)
		idx++
	}
	whereSQL, whereArgs := b.buildWhere(idx)
	args = append(args, whereArgs...)

	q := fmt.Sprintf("UPDATE %s SET %s%s", b.dialect.Quote(b.table), strings.Join(sets, ", "), whereSQL)
	res, err := b.exec.ExecContext(b.ctx, q, args...)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err == nil {
		for _, o := range obs {
			if o.Updated != nil {
				o.Updated(affected)
			}
		}
	}
	return affected, err
}

// Delete removes matching rows (or sets deleted_at if the model embeds model.SoftDeletes).
func (b *Builder[T]) Delete() (int64, error) {
	if b.err != nil {
		return 0, b.err
	}
	obs := getObservers[T]()
	for _, o := range obs {
		if o.Deleting != nil {
			if err := o.Deleting(); err != nil {
				return 0, err
			}
		}
	}
	if b.meta.softDeletes && !b.withTrashed {
		affected, err := b.Update(map[string]any{"deleted_at": time.Now().UTC()})
		if err == nil {
			for _, o := range obs {
				if o.Deleted != nil {
					o.Deleted(affected)
				}
			}
		}
		return affected, err
	}
	whereSQL, args := b.buildWhere(1)
	q := fmt.Sprintf("DELETE FROM %s%s", b.dialect.Quote(b.table), whereSQL)
	res, err := b.exec.ExecContext(b.ctx, q, args...)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err == nil {
		for _, o := range obs {
			if o.Deleted != nil {
				o.Deleted(affected)
			}
		}
	}
	return affected, err
}

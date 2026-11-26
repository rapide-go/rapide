package query

import (
	"database/sql"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/rapide-go/rapide/database/model"
)

type fieldInfo struct {
	col   string
	index []int
}

type typeMeta struct {
	table       string
	fields      []fieldInfo
	byCol       map[string][]int
	softDeletes bool
}

var metaCache sync.Map // map[reflect.Type]*typeMeta

func getTypeMeta[T any](t reflect.Type) *typeMeta {
	if v, ok := metaCache.Load(t); ok {
		return v.(*typeMeta)
	}
	m := &typeMeta{
		table: resolveTableName[T](t),
		byCol: make(map[string][]int),
	}
	if t.Kind() == reflect.Struct {
		indexFields(t, nil, m)
	}
	_, m.softDeletes = m.byCol["deleted_at"]
	actual, _ := metaCache.LoadOrStore(t, m)
	return actual.(*typeMeta)
}

func indexFields(t reflect.Type, prefix []int, m *typeMeta) {
	for i := range t.NumField() {
		sf := t.Field(i)
		idx := append(append([]int(nil), prefix...), i)
		if sf.Anonymous && sf.Type.Kind() == reflect.Struct {
			indexFields(sf.Type, idx, m)
			continue
		}
		col := sf.Tag.Get("db")
		if col == "-" {
			continue
		}
		if col == "" {
			col = toSnakeCase(sf.Name)
		}
		m.fields = append(m.fields, fieldInfo{col: col, index: idx})
		m.byCol[col] = idx
	}
}

func (m *typeMeta) hasCol(col string) bool {
	_, ok := m.byCol[col]
	return ok
}

func (m *typeMeta) scan(rows *sql.Rows, cols []string, dest any) error {
	rv := reflect.ValueOf(dest).Elem()
	ptrs := make([]any, len(cols))
	for i, col := range cols {
		if idx, ok := m.byCol[col]; ok {
			fv := rv.FieldByIndex(idx)
			if fv.CanAddr() {
				ptrs[i] = fv.Addr().Interface()
				continue
			}
		}
		var discard any
		ptrs[i] = &discard
	}
	return rows.Scan(ptrs...)
}

func (m *typeMeta) extract(item any, skipZeroID bool) ([]string, []any) {
	rv := reflect.ValueOf(item).Elem()
	cols := make([]string, 0, len(m.fields))
	vals := make([]any, 0, len(m.fields))
	for _, f := range m.fields {
		fv := rv.FieldByIndex(f.index)
		if skipZeroID && f.col == "id" && fv.IsZero() {
			continue
		}
		cols = append(cols, f.col)
		vals = append(vals, fv.Interface())
	}
	return cols, vals
}

func (m *typeMeta) setTimestamp(item any, col string, t time.Time) {
	idx, ok := m.byCol[col]
	if !ok {
		return
	}
	fv := reflect.ValueOf(item).Elem().FieldByIndex(idx)
	if fv.CanSet() && fv.Type() == reflect.TypeFor[time.Time]() && fv.IsZero() {
		fv.Set(reflect.ValueOf(t))
	}
}

func (m *typeMeta) setID(item any, id int64) {
	idx, ok := m.byCol["id"]
	if !ok {
		return
	}
	fv := reflect.ValueOf(item).Elem().FieldByIndex(idx)
	if !fv.CanSet() {
		return
	}
	switch fv.Kind() {
	case reflect.Int, reflect.Int64:
		fv.SetInt(id)
	case reflect.Uint, reflect.Uint64:
		fv.SetUint(uint64(id))
	}
}

func resolveTableName[T any](t reflect.Type) string {
	var zero T
	if tabler, ok := any(zero).(model.Tabler); ok {
		return tabler.TableName()
	}
	return toSnakeCase(t.Name()) + "s"
}

func toSnakeCase(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) && i > 0 {
			sb.WriteByte('_')
		}
		sb.WriteRune(unicode.ToLower(r))
	}
	return sb.String()
}

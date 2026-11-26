package query

import (
	"fmt"
	"strings"
)

func (b *Builder[T]) buildSelect() (string, []any) {
	cols := "*"
	if len(b.columns) > 0 {
		quoted := make([]string, len(b.columns))
		for i, c := range b.columns {
			quoted[i] = b.dialect.Quote(c)
		}
		cols = strings.Join(quoted, ", ")
	}
	whereSQL, args := b.buildWhere(1)

	var sb strings.Builder
	sb.WriteString("SELECT ")
	sb.WriteString(cols)
	sb.WriteString(" FROM ")
	sb.WriteString(b.dialect.Quote(b.table))
	sb.WriteString(whereSQL)

	if len(b.orders) > 0 {
		sb.WriteString(" ORDER BY ")
		sb.WriteString(strings.Join(b.orders, ", "))
	}
	if b.limit != nil {
		sb.WriteString(fmt.Sprintf(" LIMIT %d", *b.limit))
	}
	if b.offset != nil {
		sb.WriteString(fmt.Sprintf(" OFFSET %d", *b.offset))
	}
	return sb.String(), args
}

func (b *Builder[T]) buildWhere(startIdx int) (string, []any) {
	clauses := append([]whereClause(nil), b.wheres...)
	if b.meta.softDeletes && !b.withTrashed {
		clauses = append(clauses, whereClause{
			boolean: "AND",
			raw:     b.dialect.Quote("deleted_at") + " IS NULL",
		})
	}
	if len(clauses) == 0 {
		return "", nil
	}

	var sb strings.Builder
	var args []any
	idx := startIdx
	sb.WriteString(" WHERE ")
	for i, c := range clauses {
		if i > 0 {
			sb.WriteByte(' ')
			sb.WriteString(c.boolean)
			sb.WriteByte(' ')
		}
		if c.raw != "" {
			sb.WriteString(c.raw)
			continue
		}
		if c.op == "IN" {
			phs := make([]string, len(c.args))
			for j := range c.args {
				phs[j] = b.dialect.Placeholder(idx)
				idx++
			}
			sb.WriteString(fmt.Sprintf("%s IN (%s)", b.dialect.Quote(c.col), strings.Join(phs, ", ")))
			args = append(args, c.args...)
			continue
		}
		sb.WriteString(fmt.Sprintf("%s %s %s", b.dialect.Quote(c.col), c.op, b.dialect.Placeholder(idx)))
		idx++
		args = append(args, c.args...)
	}
	return sb.String(), args
}

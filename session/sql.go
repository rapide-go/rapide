package session

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/database/schema"
)

type sessionRecord struct {
	ID        string `db:"id"`
	Payload   string `db:"payload"`
	ExpiresAt int64  `db:"expires_at"`
}

func (sessionRecord) TableName() string { return "sessions" }

// SQLStore persists sessions in any SQL database using Rapide's schema builder and generic ORM.
type SQLStore struct {
	db      *sql.DB
	dialect dialect.Dialect
	table   string
}

// NewSQLStore ensures the sessions table exists via database/schema and returns a Store.
func NewSQLStore(db *sql.DB, d dialect.Dialect, table ...string) (*SQLStore, error) {
	if d == nil {
		d = query.DefaultDialect
	}
	tbl := "sessions"
	if len(table) > 0 && table[0] != "" {
		tbl = table[0]
	}
	err := schema.NewBuilder(db, d).Create(tbl, func(t *schema.Table) {
		t.String("id").Unique()
		t.Text("payload")
		t.Integer("expires_at")
	})
	if err != nil {
		return nil, err
	}
	return &SQLStore{db: db, dialect: d, table: tbl}, nil
}

func (s *SQLStore) q(c context.Context) *query.Builder[sessionRecord] {
	return query.QueryOn[sessionRecord](s.db).WithContext(c).WithDialect(s.dialect).Table(s.table)
}

func (s *SQLStore) Load(c context.Context, id string) (map[string]any, time.Time, bool, error) {
	rec, err := s.q(c).Where("id", "=", id).First()
	if err == query.ErrNotFound {
		return nil, time.Time{}, false, nil
	}
	if err != nil {
		return nil, time.Time{}, false, err
	}
	exp := time.Unix(rec.ExpiresAt, 0)
	if time.Now().After(exp) {
		_ = s.Delete(c, id)
		return nil, time.Time{}, false, nil
	}
	data := make(map[string]any)
	if rec.Payload != "" {
		if err := json.Unmarshal([]byte(rec.Payload), &data); err != nil {
			return nil, time.Time{}, false, err
		}
	}
	return data, exp, true, nil
}

func (s *SQLStore) Save(c context.Context, id string, data map[string]any, expiresAt time.Time) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, _ = s.q(c).Where("id", "=", id).Delete()
	_, err = s.q(c).Create(sessionRecord{
		ID:        id,
		Payload:   string(raw),
		ExpiresAt: expiresAt.Unix(),
	})
	return err
}

func (s *SQLStore) Delete(c context.Context, id string) error {
	_, err := s.q(c).Where("id", "=", id).Delete()
	return err
}

func (s *SQLStore) Sweep(c context.Context, now time.Time) error {
	_, err := s.q(c).Where("expires_at", "<", now.Unix()).Delete()
	return err
}

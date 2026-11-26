package cache

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rapide-go/rapide/database/dialect"
)

// SQLStore is a persistent database-backed cache Store.
// Works with SQLite, PostgreSQL, or MySQL with zero Redis required.
type SQLStore struct {
	db    *sql.DB
	table string
}

// NewSQLStore initializes a SQL-backed cache store and creates its backing table if missing.
func NewSQLStore(db *sql.DB, table ...string) (*SQLStore, error) {
	tbl := "_cache"
	if len(table) > 0 && table[0] != "" {
		tbl = table[0]
	}
	if err := dialect.ValidateIdentifier(tbl); err != nil {
		return nil, err
	}
	s := &SQLStore{db: db, table: tbl}
	if err := s.ensureTable(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *SQLStore) ensureTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS ` + s.table + ` (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			expires_at INTEGER NOT NULL DEFAULT 0
		)`)
	return err
}

func (s *SQLStore) Get(key string) (any, bool) {
	var (
		raw string
		exp int64
	)
	err := s.db.QueryRow("SELECT value, expires_at FROM "+s.table+" WHERE key = ?", key).Scan(&raw, &exp)
	if err != nil {
		return nil, false
	}
	if exp > 0 && time.Now().Unix() > exp {
		s.Forget(key)
		return nil, false
	}
	var val any
	if err := json.Unmarshal([]byte(raw), &val); err != nil {
		return raw, true
	}
	return val, true
}

func (s *SQLStore) Put(key string, value any, ttl time.Duration) {
	raw, err := json.Marshal(value)
	if err != nil {
		return
	}
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).Unix()
	}
	_, _ = s.db.Exec(
		"INSERT INTO "+s.table+" (key, value, expires_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, expires_at = excluded.expires_at",
		key, string(raw), exp,
	)
}

func (s *SQLStore) Forget(key string) {
	_, _ = s.db.Exec("DELETE FROM "+s.table+" WHERE key = ?", key)
}

// Touch extends key's expires_at in SQL without fetching or unmarshaling its payload.
func (s *SQLStore) Touch(key string, ttl time.Duration) bool {
	var exp int64
	if ttl > 0 {
		exp = time.Now().Add(ttl).Unix()
	}
	res, err := s.db.Exec(
		"UPDATE "+s.table+" SET expires_at = ? WHERE key = ? AND (expires_at = 0 OR expires_at >= ?)",
		exp, key, time.Now().Unix(),
	)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n > 0
}

func (s *SQLStore) Flush() {
	_, _ = s.db.Exec("DELETE FROM " + s.table)
}

// Sweep purges all expired rows from the cache table.
func (s *SQLStore) Sweep() {
	_, _ = s.db.Exec("DELETE FROM "+s.table+" WHERE expires_at > 0 AND expires_at < ?", time.Now().Unix())
}

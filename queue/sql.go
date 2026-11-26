package queue

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
	"time"

	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/database/schema"
)

// StoredJob is the persistent ORM model backing the `jobs` table.
type StoredJob struct {
	ID          int64  `db:"id"`
	Queue       string `db:"queue"`
	Name        string `db:"name"`
	Payload     string `db:"payload"`
	Attempts    int    `db:"attempts"`
	MaxAttempts int    `db:"max_attempts"`
	AvailableAt int64  `db:"available_at"`
	CreatedAt   int64  `db:"created_at"`
}

func (StoredJob) TableName() string { return "jobs" }

// FailedJob is the persistent ORM model backing the `failed_jobs` dead-letter table.
type FailedJob struct {
	ID       int64  `db:"id"`
	Queue    string `db:"queue"`
	Name     string `db:"name"`
	Payload  string `db:"payload"`
	Error    string `db:"error"`
	FailedAt int64  `db:"failed_at"`
}

func (FailedJob) TableName() string { return "failed_jobs" }

// SQLStore provides a persistent SQL-backed queue with delayed jobs and `failed_jobs`,
// built on Rapide's database/schema and database/query ORM.
type SQLStore struct {
	db      *sql.DB
	dialect dialect.Dialect

	mu       sync.RWMutex
	handlers map[string]func(ctx context.Context, payload []byte) error
}

// NewSQLStore ensures `jobs` and `failed_jobs` exist via database/schema and returns a SQLStore.
func NewSQLStore(db *sql.DB, d dialect.Dialect) (*SQLStore, error) {
	if d == nil {
		d = query.DefaultDialect
	}
	sb := schema.NewBuilder(db, d)
	if err := sb.Create("jobs", func(t *schema.Table) {
		t.ID()
		t.String("queue")
		t.String("name")
		t.Text("payload")
		t.Integer("attempts").Default("0")
		t.Integer("max_attempts").Default("3")
		t.Integer("available_at")
		t.Integer("created_at")
	}); err != nil {
		return nil, err
	}
	if err := sb.Create("failed_jobs", func(t *schema.Table) {
		t.ID()
		t.String("queue")
		t.String("name")
		t.Text("payload")
		t.Text("error")
		t.Integer("failed_at")
	}); err != nil {
		return nil, err
	}
	return &SQLStore{
		db:       db,
		dialect:  d,
		handlers: make(map[string]func(ctx context.Context, payload []byte) error),
	}, nil
}

func (s *SQLStore) jobs(c context.Context) *query.Builder[StoredJob] {
	return query.QueryOn[StoredJob](s.db).WithContext(c).WithDialect(s.dialect)
}

func (s *SQLStore) failed(c context.Context) *query.Builder[FailedJob] {
	return query.QueryOn[FailedJob](s.db).WithContext(c).WithDialect(s.dialect)
}

// Handle binds a typed JSON payload handler to a persistent job name (Go 1.27 generic method).
func (s *SQLStore) Handle[P any](name string, fn func(ctx context.Context, payload P) error) {
	s.mu.Lock()
	s.handlers[name] = func(ctx context.Context, raw []byte) error {
		var p P
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return err
			}
		}
		return fn(ctx, p)
	}
	s.mu.Unlock()
}

// RegisterHandler delegates to s.Handle (kept for backwards compatibility).
func RegisterHandler[P any](s *SQLStore, name string, fn func(ctx context.Context, payload P) error) {
	s.Handle(name, fn)
}

// Push persists a job immediately onto queueName (default "default").
func (s *SQLStore) Push(ctx context.Context, queueName, name string, payload any, maxAttempts int) error {
	return s.PushAfter(ctx, 0, queueName, name, payload, maxAttempts)
}

// PushAfter persists a delayed job that becomes eligible after delay.
func (s *SQLStore) PushAfter(ctx context.Context, delay time.Duration, queueName, name string, payload any, maxAttempts int) error {
	if queueName == "" {
		queueName = "default"
	}
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	_, err = s.jobs(ctx).Create(StoredJob{
		Queue:       queueName,
		Name:        name,
		Payload:     string(raw),
		MaxAttempts: maxAttempts,
		AvailableAt: now.Add(delay).Unix(),
		CreatedAt:   now.Unix(),
	})
	return err
}

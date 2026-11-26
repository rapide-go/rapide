package notify

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/rapide-go/rapide/container"
)

// DatabaseNotification is implemented by notifications persisted to the `_notifications` table.
type DatabaseNotification interface {
	ToDatabase(recipient Notifiable) (notifiableID string, notifType string, data map[string]any)
}

// WebhookNotification is implemented by notifications delivered via HTTP POST (Slack/Discord/custom).
type WebhookNotification interface {
	ToWebhook(recipient Notifiable) (url string, payload map[string]any)
}

// StoredNotification represents a persisted in-app notification row.
type StoredNotification struct {
	ID           int64          `json:"id"`
	NotifiableID string         `json:"notifiable_id"`
	Type         string         `json:"type"`
	Data         map[string]any `json:"data"`
	ReadAt       *time.Time     `json:"read_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

// Store persists in-app notifications in a SQL table (`_notifications`).
type Store struct {
	db *sql.DB
}

// NewStore creates a notification Store and ensures the `_notifications` table exists.
func NewStore(db *sql.DB) (*Store, error) {
	s := &Store{db: db}
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _notifications (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			notifiable_id TEXT NOT NULL,
			type TEXT NOT NULL,
			data TEXT NOT NULL,
			read_at TIMESTAMP NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`)
	return s, err
}

// Record inserts a new unread notification row.
func (s *Store) Record(ctx context.Context, notifiableID, notifType string, data map[string]any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO _notifications (notifiable_id, type, data) VALUES (?, ?, ?)",
		notifiableID, notifType, string(raw),
	)
	return err
}

// UnreadFor returns all unread notifications for notifiableID newest first.
func (s *Store) UnreadFor(ctx context.Context, notifiableID string) ([]StoredNotification, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, notifiable_id, type, data, created_at FROM _notifications WHERE notifiable_id = ? AND read_at IS NULL ORDER BY id DESC",
		notifiableID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StoredNotification
	for rows.Next() {
		var (
			n   StoredNotification
			raw string
		)
		if err := rows.Scan(&n.ID, &n.NotifiableID, &n.Type, &raw, &n.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(raw), &n.Data)
		out = append(out, n)
	}
	return out, rows.Err()
}

// MarkAsRead marks a notification row as read.
func (s *Store) MarkAsRead(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, "UPDATE _notifications SET read_at = ? WHERE id = ?", time.Now().UTC(), id)
	return err
}

// SetStore binds the default notification *Store into the Rapide container.
func SetStore(s *Store) {
	container.Register[*Store](s)
}

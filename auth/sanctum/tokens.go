package sanctum

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rapide-go/rapide/container"
	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/query"
	"github.com/rapide-go/rapide/database/schema"
	"github.com/rapide-go/rapide/foundation"
)

var (
	ErrInvalidToken = errors.New("sanctum: invalid personal access token")
	ErrExpiredToken = errors.New("sanctum: personal access token expired")
)

// AccessToken is the ORM model backing `personal_access_tokens`.
type AccessToken struct {
	ID            int64    `db:"id"`
	UserID        string   `db:"user_id"`
	Name          string   `db:"name"`
	TokenHash     string   `db:"token_hash"`
	AbilitiesJSON string   `db:"abilities"`
	ExpiresAtUnix int64    `db:"expires_at"`
	LastUsedUnix  int64    `db:"last_used_at"`
	CreatedAtUnix int64    `db:"created_at"`
	Abilities     []string `db:"-"`
}

func (AccessToken) TableName() string { return "personal_access_tokens" }

// Can reports whether this token grants ability (or wildcard "*").
func (t *AccessToken) Can(ability string) bool {
	if t == nil {
		return false
	}
	return slices.Contains(t.Abilities, "*") || slices.Contains(t.Abilities, ability)
}

// Manager persists and validates revocable personal access tokens using Rapide's schema + ORM.
type Manager struct {
	db      *sql.DB
	dialect dialect.Dialect
}

// NewManager ensures `personal_access_tokens` exists via database/schema and returns a Manager.
func NewManager(db *sql.DB, d dialect.Dialect) (*Manager, error) {
	if d == nil {
		d = query.DefaultDialect
	}
	err := schema.NewBuilder(db, d).Create("personal_access_tokens", func(t *schema.Table) {
		t.ID()
		t.String("user_id")
		t.String("name")
		t.String("token_hash").Unique()
		t.Text("abilities")
		t.Integer("expires_at").Default("0")
		t.Integer("last_used_at").Default("0")
		t.Integer("created_at")
	})
	if err != nil {
		return nil, err
	}
	return &Manager{db: db, dialect: d}, nil
}

func (m *Manager) q(c context.Context) *query.Builder[AccessToken] {
	return query.QueryOn[AccessToken](m.db).WithContext(c).WithDialect(m.dialect)
}

// CreateToken issues a new personal access token and returns (plainText "id|secret", *AccessToken, error).
func (m *Manager) CreateToken(c context.Context, userID, name string, abilities []string, ttl time.Duration) (string, *AccessToken, error) {
	if len(abilities) == 0 {
		abilities = []string{"*"}
	}
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", nil, err
	}
	secret := hex.EncodeToString(secretBytes)
	abilitiesRaw, err := json.Marshal(abilities)
	if err != nil {
		return "", nil, err
	}

	now := time.Now().UTC()
	var expUnix int64
	if ttl > 0 {
		expUnix = now.Add(ttl).Unix()
	}

	created, err := m.q(c).Create(AccessToken{
		UserID:        userID,
		Name:          name,
		TokenHash:     hashToken(secret),
		AbilitiesJSON: string(abilitiesRaw),
		ExpiresAtUnix: expUnix,
		CreatedAtUnix: now.Unix(),
	})
	if err != nil {
		return "", nil, err
	}
	created.Abilities = abilities
	return fmt.Sprintf("%d|%s", created.ID, secret), &created, nil
}

// Verify validates an `id|secret` or raw `secret` bearer token and updates last_used_at.
func (m *Manager) Verify(c context.Context, plainText string) (*AccessToken, error) {
	plainText = strings.TrimSpace(plainText)
	if plainText == "" {
		return nil, ErrInvalidToken
	}

	var idPart int64
	secret := plainText
	if left, right, ok := strings.Cut(plainText, "|"); ok {
		parsedID, err := strconv.ParseInt(left, 10, 64)
		if err != nil {
			return nil, ErrInvalidToken
		}
		idPart = parsedID
		secret = right
	}

	wantHash := hashToken(secret)
	var (
		tok AccessToken
		err error
	)
	if idPart > 0 {
		tok, err = m.q(c).Find(idPart)
	} else {
		tok, err = m.q(c).Where("token_hash", "=", wantHash).First()
	}
	if err == query.ErrNotFound {
		return nil, ErrInvalidToken
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(tok.TokenHash), []byte(wantHash)) != 1 {
		return nil, ErrInvalidToken
	}

	now := time.Now().UTC().Unix()
	if tok.ExpiresAtUnix > 0 && now > tok.ExpiresAtUnix {
		return nil, ErrExpiredToken
	}
	_ = json.Unmarshal([]byte(tok.AbilitiesJSON), &tok.Abilities)
	tok.LastUsedUnix = now
	_, _ = m.q(c).Where("id", "=", tok.ID).Update(map[string]any{"last_used_at": now})
	return &tok, nil
}

// Revoke deletes a single personal access token by ID.
func (m *Manager) Revoke(c context.Context, tokenID int64) error {
	_, err := m.q(c).Where("id", "=", tokenID).Delete()
	return err
}

// RevokeAll deletes every personal access token belonging to userID.
func (m *Manager) RevokeAll(c context.Context, userID string) error {
	_, err := m.q(c).Where("user_id", "=", userID).Delete()
	return err
}

// Default returns the container-bound *Manager.
func Default() *Manager {
	return container.Use[*Manager]()
}

// Provider registers a Sanctum *Manager into the Rapide container.
type Provider struct {
	foundation.BaseProvider
	DB      *sql.DB
	Dialect dialect.Dialect
}

func (p Provider) Register(app *foundation.App) error {
	db := p.DB
	if db == nil {
		db = query.DB()
	}
	m, err := NewManager(db, p.Dialect)
	if err != nil {
		return err
	}
	container.Bind[*Manager](app.Container, m)
	return nil
}

func hashToken(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

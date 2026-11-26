package query

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rapide-go/rapide/config"
	"github.com/rapide-go/rapide/database/dialect"
	"github.com/rapide-go/rapide/database/schema"
	"github.com/rapide-go/rapide/foundation"
)

// Provider opens the default SQL database from config (`database.driver`, `database.dsn`),
// configures safe connection-pool limits, verifies connectivity on startup, applies SQLite
// WAL/FK pragmas, selects the matching SQL dialect, binds the DB into the container,
// runs pending migrations on Boot, and closes the connection cleanly on shutdown.
type Provider struct {
	foundation.BaseProvider
	Driver          string
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	Migrations      []schema.Migration
	AutoMigrate     bool
}

func (p Provider) Register(app *foundation.App) error {
	driver := p.Driver
	if driver == "" {
		driver = config.Get("database.driver", "sqlite")
	}
	dsn := p.DSN
	if dsn == "" {
		dsn = config.Get("database.dsn", "storage/database.sqlite")
	}

	isSQLite := false
	switch strings.ToLower(driver) {
	case "postgres", "pgx", "postgresql":
		SetDialect(dialect.Postgres{})
	case "mysql":
		SetDialect(dialect.MySQL{})
	default:
		isSQLite = true
		SetDialect(dialect.SQLite{})
		if dsn != ":memory:" && !strings.HasPrefix(dsn, "file:") {
			if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
				return err
			}
		}
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}

	// Production-safe connection pool defaults (SQLite single-writer vs Postgres/MySQL pool).
	if isSQLite {
		maxOpen := p.MaxOpenConns
		if maxOpen <= 0 {
			maxOpen = 1
		}
		db.SetMaxOpenConns(maxOpen)
		db.SetMaxIdleConns(maxOpen)
		db.SetConnMaxLifetime(0)
	} else {
		maxOpen := p.MaxOpenConns
		if maxOpen <= 0 {
			maxOpen = 25
		}
		maxIdle := p.MaxIdleConns
		if maxIdle <= 0 {
			maxIdle = 10
		}
		lifetime := p.ConnMaxLifetime
		if lifetime <= 0 {
			lifetime = 5 * time.Minute
		}
		db.SetMaxOpenConns(maxOpen)
		db.SetMaxIdleConns(maxIdle)
		db.SetConnMaxLifetime(lifetime)
		db.SetConnMaxIdleTime(2 * time.Minute)
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return fmt.Errorf("database: ping %s: %w", driver, err)
	}

	if isSQLite {
		_, _ = db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;")
	}

	SetDB(db)
	app.OnShutdown(func(context.Context) error {
		return db.Close()
	})
	return nil
}

func (p Provider) Boot(app *foundation.App) error {
	if len(p.Migrations) == 0 && !p.AutoMigrate {
		return nil
	}
	all := append(append([]schema.Migration(nil), p.Migrations...), foundation.Extensions[schema.Migration](app)...)
	if len(all) == 0 {
		return nil
	}
	_, err := schema.NewMigrator(DB(), all...).Up(context.Background())
	return err
}

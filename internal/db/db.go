package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"modernc.org/sqlite"

	"github.com/tta-lab/ttal-cli/ent"
	"github.com/tta-lab/ttal-cli/ent/migrate"
	"github.com/tta-lab/ttal-cli/internal/config"
)

type DB struct {
	*ent.Client
}

// DefaultPath returns the default database path for the active team.
func DefaultPath() string {
	return config.ResolveDBPath()
}

func init() {
	sqlite.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, dsn string) error {
		// Enable foreign key constraints
		if _, err := conn.ExecContext(context.Background(), "PRAGMA foreign_keys = ON", nil); err != nil {
			return fmt.Errorf("failed to enable foreign keys: %w", err)
		}
		// Use Write-Ahead Logging for better concurrency; fall back to DELETE
		// mode when WAL is unavailable (e.g. inside a sandbox that blocks mmap).
		if _, err := conn.ExecContext(context.Background(), "PRAGMA journal_mode = WAL", nil); err != nil {
			if _, err2 := conn.ExecContext(context.Background(), "PRAGMA journal_mode = DELETE", nil); err2 != nil {
				return fmt.Errorf("failed to set journal mode: WAL: %v, DELETE: %w", err, err2)
			}
		}
		// Wait up to 5 seconds on lock conflicts
		if _, err := conn.ExecContext(context.Background(), "PRAGMA busy_timeout = 5000", nil); err != nil {
			return fmt.Errorf("failed to set busy timeout: %w", err)
		}
		return nil
	})
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open with modernc.org/sqlite driver, then wrap for ent
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s", dbPath))
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := ent.NewClient(ent.Driver(drv))

	// Run auto-migrations
	if err := client.Schema.Create(context.Background(), migrate.WithDropColumn(true)); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{client}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.Client.Close()
}

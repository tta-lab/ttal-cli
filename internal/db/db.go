package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/guion-opensource/ttal-cli/ent"
	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*ent.Client
}

// New creates a new database connection and runs migrations
func New(dbPath string) (*DB, error) {
	// Ensure directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open ent client with SQLite and recommended settings
	// - cache=shared: Enable shared cache for better concurrency
	// - _fk=1: Enable foreign key constraints
	// - _journal_mode=WAL: Use Write-Ahead Logging for better performance
	// - _busy_timeout=5000: Wait up to 5 seconds on lock conflicts
	dsn := fmt.Sprintf("file:%s?cache=shared&_fk=1&_journal_mode=WAL&_busy_timeout=5000", dbPath)
	client, err := ent.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Run auto-migrations
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return &DB{client}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.Client.Close()
}

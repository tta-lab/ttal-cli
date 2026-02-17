package db

import (
	"context"
	"testing"

	"codeberg.org/clawteam/ttal-cli/ent"
	_ "github.com/mattn/go-sqlite3"
)

// NewTestDB creates an in-memory SQLite database for testing
func NewTestDB(t *testing.T) *DB {
	t.Helper()

	// Use in-memory SQLite database
	// Each connection gets its own isolated in-memory database
	client, err := ent.Open("sqlite3", "file::memory:?cache=shared&_fk=1")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Run auto-migrations
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	db := &DB{client}

	// Register cleanup
	t.Cleanup(func() {
		_ = db.Close()
	})

	return db
}

package db

import (
	"context"
	"database/sql"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"

	"github.com/tta-lab/ttal-cli/ent"
)

// NewTestDB creates an in-memory SQLite database for testing
func NewTestDB(t *testing.T) *DB {
	t.Helper()

	// Use in-memory SQLite database via modernc driver
	sqlDB, err := sql.Open("sqlite", "file::memory:")
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	drv := entsql.OpenDB(dialect.SQLite, sqlDB)
	client := ent.NewClient(ent.Driver(drv))

	// Run auto-migrations
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		t.Fatalf("failed to run migrations: %v", err)
	}

	d := &DB{client}

	// Register cleanup
	t.Cleanup(func() {
		_ = d.Close()
	})

	return d
}

package usage

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/ent"
	_ "modernc.org/sqlite"
)

// Log records a command invocation. Skips silently if TTAL_AGENT_NAME is not
// set or if the database is unreachable. Never fails the caller.
func Log(subcommand, target string) {
	agent := os.Getenv("TTAL_AGENT_NAME")
	if agent == "" {
		return
	}
	team := os.Getenv("TTAL_TEAM")
	if team == "" {
		team = "default"
	}

	client, err := open()
	if err != nil {
		return
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	builder := client.ToolUsage.Create().
		SetAgent(agent).
		SetTeam(team).
		SetCommand("ttal").
		SetSubcommand(subcommand)
	if target != "" {
		builder = builder.SetTarget(target)
	}
	if _, err := builder.Save(ctx); err != nil {
		log.Printf("[usage] log error: %v", err)
	}
}

func open() (*ent.Client, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dbPath := filepath.Join(home, ".ttal", "messages.db")
	dbDSN := "file:" + dbPath + "?cache=shared&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"

	drv, err := entsql.Open("sqlite", dbDSN)
	if err != nil {
		return nil, err
	}

	client := ent.NewClient(ent.Driver(entsql.OpenDB("sqlite3", drv.DB())))

	// Ensure schema exists (idempotent, fast for existing tables).
	if err := client.Schema.Create(context.Background()); err != nil {
		_ = client.Close()
		return nil, err
	}

	return client, nil
}

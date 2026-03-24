package usage

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/ent"
	_ "modernc.org/sqlite"
)

// LogWith records a tool invocation. command is the tool name (e.g. "ttal", "flicknote").
// Skips silently if TTAL_AGENT_NAME is not set. Never fails the caller.
func LogWith(command, subcommand, target string) {
	agent := os.Getenv("TTAL_AGENT_NAME")
	if agent == "" {
		return
	}
	team := config.DefaultTeamName

	client, err := open()
	if err != nil {
		log.Printf("[usage] open error: %v", err)
		return
	}
	defer func() { _ = client.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	builder := client.ToolUsage.Create().
		SetAgent(agent).
		SetTeam(team).
		SetCommand(command).
		SetSubcommand(subcommand)
	if target != "" {
		builder = builder.SetTarget(target)
	}
	if _, err := builder.Save(ctx); err != nil {
		log.Printf("[usage] log error: %v", err)
	}
}

// Log records a ttal command invocation with "command attempted" semantics — it fires
// immediately after flag validation, before the underlying operation completes.
// Convenience wrapper for LogWith("ttal", ...).
// Skips silently if TTAL_AGENT_NAME is not set. Never fails the caller.
func Log(subcommand, target string) {
	LogWith("ttal", subcommand, target)
}

// open returns an ent client connected to ~/.ttal/messages.db.
// Schema creation is owned by the daemon — this function does not migrate.
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
	return ent.NewClient(ent.Driver(drv)), nil
}

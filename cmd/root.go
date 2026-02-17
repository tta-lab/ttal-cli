package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"codeberg.org/clawteam/ttal-cli/internal/db"
	"github.com/spf13/cobra"
)

var (
	dbPath   string
	database *db.DB
)

var rootCmd = &cobra.Command{
	Use:   "ttal",
	Short: "TTAL - Task & Team Agent Lifecycle Manager",
	Long: `TTAL is a CLI tool for managing projects, agents, and automated memory capture.
It provides taskwarrior-like syntax for tag management and agent routing.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize database connection
		var err error
		database, err = db.New(dbPath)
		if err != nil {
			return fmt.Errorf("failed to initialize database: %w", err)
		}
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		if database != nil {
			return database.Close()
		}
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Get default database path
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	defaultDBPath := filepath.Join(home, ".ttal", "ttal.db")

	rootCmd.PersistentFlags().StringVar(&dbPath, "db", defaultDBPath, "Path to SQLite database")
}

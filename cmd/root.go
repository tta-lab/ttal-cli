package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/db"
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
		// Load .env as fallback for tokens not already in the environment
		if dotEnv, err := config.LoadDotEnv(); err == nil {
			for k, v := range dotEnv {
				if os.Getenv(k) == "" {
					_ = os.Setenv(k, v)
				}
			}
		}

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
	rootCmd.PersistentFlags().StringVar(&dbPath, "db", db.DefaultPath(), "Path to SQLite database")
}

// confirmPrompt asks the user a yes/no question and returns true if they answer "y".
func confirmPrompt(message string) bool {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	return strings.ToLower(strings.TrimSpace(answer)) == "y"
}

// deleteEntity checks existence, confirms with user, then deletes.
// existFn checks if the entity exists, deleteFn performs the deletion.
func deleteEntity(kind, name string, existFn func() (bool, error), deleteFn func() (int, error)) error {
	exists, err := existFn()
	if err != nil {
		return fmt.Errorf("failed to query %s: %w", kind, err)
	}
	if !exists {
		return fmt.Errorf("%s '%s' not found", kind, name)
	}

	if !confirmPrompt(fmt.Sprintf("Permanently delete %s '%s'? [y/N] ", kind, name)) {
		fmt.Println("Aborted.")
		return nil
	}

	count, err := deleteFn()
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", kind, err)
	}
	if count == 0 {
		return fmt.Errorf("%s '%s' not found", kind, name)
	}

	fmt.Printf("%s '%s' deleted permanently\n", strings.ToUpper(kind[:1])+kind[1:], name)
	return nil
}

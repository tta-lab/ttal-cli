package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/memory"
	"github.com/spf13/cobra"
)

var (
	memoryDate      string
	memoryOutputDir string
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory capture",
	Long:  `Capture and generate agent memory logs from git commits.`,
}

var memoryCaptureCmd = &cobra.Command{
	Use:   "capture",
	Short: "Capture memory from git commits",
	Long: `Scan all active projects and generate agent-filtered memory logs.

Examples:
  ttal memory capture --date=2026-02-08
  ttal memory capture --date=2026-02-08 --output=/path/to/memory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Parse date
		var date time.Time
		var err error
		if memoryDate == "" {
			date = time.Now()
		} else {
			date, err = time.Parse("2006-01-02", memoryDate)
			if err != nil {
				return fmt.Errorf("invalid date format (use YYYY-MM-DD): %w", err)
			}
		}

		// Set default output directory
		if memoryOutputDir == "" {
			memoryOutputDir = filepath.Join(config.ResolveDataDir(), "memory")
		}

		// Create capturer
		capturer := memory.NewCapturer(database.Client)

		// Capture memory
		fmt.Printf("Capturing memory for %s...\n", date.Format("2006-01-02"))
		if err := capturer.Capture(date, memoryOutputDir); err != nil {
			return fmt.Errorf("failed to capture memory: %w", err)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(memoryCmd)
	memoryCmd.AddCommand(memoryCaptureCmd)

	// Flags for memory capture
	memoryCaptureCmd.Flags().StringVar(&memoryDate, "date", "", "Date to capture (YYYY-MM-DD, defaults to today)")
	memoryCaptureCmd.Flags().StringVar(&memoryOutputDir, "output", "", "Output directory for memory files")
}

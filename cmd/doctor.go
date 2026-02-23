package cmd

import (
	"fmt"

	"codeberg.org/clawteam/ttal-cli/internal/doctor"
	"github.com/spf13/cobra"
)

var fixFlag bool

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check ttal setup and prerequisites",
	Long: `Validate the entire ttal setup — prerequisites, config, taskwarrior UDAs,
daemon, environment variables, and optional services.

Red/green output. First thing after install, first thing when debugging.

Use --fix to auto-create missing config template and taskwarrior UDAs.`,
	// Override root's PersistentPreRunE/PostRunE — doctor checks DB existence
	// itself and does not open a connection.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		report := doctor.Run(fixFlag)
		doctor.Print(report)
		if report.Errors() > 0 {
			cmd.SilenceErrors = true
			return fmt.Errorf("%d check(s) failed", report.Errors())
		}
		return nil
	},
}

func init() {
	doctorCmd.Flags().BoolVar(&fixFlag, "fix", false, "Auto-fix missing config and taskwarrior UDAs")
	rootCmd.AddCommand(doctorCmd)
}

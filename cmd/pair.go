package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// exportTaskByHexIDFn allows test injection of the task lookup function.
var exportTaskByHexIDFn = taskwarrior.ExportTaskByHexID

// pairCmd outputs the owner-aware pairing line for the current session.
var pairCmd = &cobra.Command{
	Use:   "pair",
	Short: "Output owner-aware pairing line for current session",
	Long: `Output a single line describing who this session is paired with.

Worker session (TTAL_JOB_ID set): pairs with the task owner (the manager that
dispatched this work).

Manager session: pairs with the admin human from humans.toml.

If the target is empty, outputs nothing and exits 0.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		var target string

		if jobID := os.Getenv("TTAL_JOB_ID"); jobID != "" {
			task, err := exportTaskByHexIDFn(jobID, "")
			if err != nil {
				log.Printf("[pair] task lookup failed: %v", err)
				return nil
			}
			target = task.Owner
		} else {
			cfg, err := config.Load()
			if err != nil {
				log.Printf("[pair] config load failed: %v", err)
				return nil
			}
			if cfg.AdminHuman != nil {
				target = cfg.AdminHuman.Alias
			}
		}

		if target == "" {
			return nil
		}

		fmt.Printf("Pairing with **%s**. Reach via `ttal send --to %s \"...\"`.\n", target, target)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(pairCmd)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/license"
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Show license tier and status",
	Long: `Show the current license tier and limits.

Tiers:
  Free  — 1 team, 2 agents, unlimited workers (default, no license needed)
  Pro   — 1 team, unlimited agents ($100 lifetime)
  Team  — unlimited teams, unlimited agents ($200 lifetime)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		lic, err := license.Load()
		if err != nil {
			return err
		}

		fmt.Printf("Tier:       %s\n", lic.Tier)

		if lic.Claims != nil {
			fmt.Printf("Licensed:   %s\n", lic.Claims.Sub)
		}

		agents := lic.MaxAgents()
		teams := lic.MaxTeams()

		if agents == -1 {
			fmt.Println("Agents:     unlimited")
		} else {
			fmt.Printf("Agents:     %d\n", agents)
		}
		if teams == -1 {
			fmt.Println("Teams:      unlimited")
		} else {
			fmt.Printf("Teams:      %d\n", teams)
		}
		fmt.Println("Workers:    unlimited")

		return nil
	},
}

var licenseActivateCmd = &cobra.Command{
	Use:   "activate <jwt>",
	Short: "Activate a license",
	Long: `Activate a Pro or Team license by providing the JWT token.

Example:
  ttal license activate eyJhbGciOi...`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := license.Activate(args[0]); err != nil {
			return fmt.Errorf("activation failed: %w", err)
		}

		lic, err := license.Load()
		if err != nil {
			return err
		}

		fmt.Printf("License activated: %s tier\n", lic.Tier)
		if lic.Claims != nil {
			fmt.Printf("Licensed to: %s\n", lic.Claims.Sub)
		}
		return nil
	},
}

var licenseDeactivateCmd = &cobra.Command{
	Use:   "deactivate",
	Short: "Remove active license (revert to free tier)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := license.LicensePath()
		if err != nil {
			return err
		}

		if !confirmPrompt(fmt.Sprintf("Remove license at %s? [y/N] ", path)) {
			fmt.Println("Aborted.")
			return nil
		}

		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove license: %w", err)
		}

		fmt.Println("License removed. Reverted to free tier.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(licenseCmd)
	licenseCmd.AddCommand(licenseActivateCmd)
	licenseCmd.AddCommand(licenseDeactivateCmd)
}

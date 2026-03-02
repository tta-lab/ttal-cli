package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/ent"
	"github.com/tta-lab/ttal-cli/ent/agent"
	"github.com/tta-lab/ttal-cli/internal/license"
	"github.com/tta-lab/ttal-cli/internal/voice"
)

var (
	agentVoice       string
	agentEmoji       string
	agentDescription string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
	Long:  `Add, list, get, and modify agents.`,
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Add a new agent",
	Long: `Add a new agent to the database.

Example:
  ttal agent add yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])

		// Enforce agent limit based on license tier.
		lic, err := license.Load()
		if err != nil {
			return fmt.Errorf("license check: %w", err)
		}
		count, err := database.Agent.Query().Count(ctx)
		if err != nil {
			return fmt.Errorf("count agents: %w", err)
		}
		if err := lic.CheckAgentLimit(count); err != nil {
			return err
		}

		creator := database.Agent.Create().
			SetName(name)

		if agentVoice != "" {
			if !voice.IsValidVoice(agentVoice) {
				return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", agentVoice)
			}
			creator = creator.SetVoice(agentVoice)
		}
		if agentEmoji != "" {
			creator = creator.SetEmoji(agentEmoji)
		}
		if agentDescription != "" {
			creator = creator.SetDescription(agentDescription)
		}

		if _, err = creator.Save(ctx); err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Printf("Agent '%s' created successfully\n", name)
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List agents",
	Long: `List all agents.

Examples:
  ttal agent list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		agents, err := database.Agent.Query().All(ctx)
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		if len(agents) == 0 {
			fmt.Println("No agents found")
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION")
		for _, a := range agents {
			name := a.Name
			if a.Emoji != "" {
				name = a.Emoji + " " + a.Name
			}

			desc := a.Description

			_, _ = fmt.Fprintf(w, "%s\t%s\n", name, desc)
		}
		_ = w.Flush()

		return nil
	},
}

var agentInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Get agent details",
	Long: `Get detailed information about an agent.

Example:
  ttal agent info yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])

		ag, err := database.Agent.Query().
			Where(agent.Name(name)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("agent '%s' not found", name)
			}
			return fmt.Errorf("failed to get agent: %w", err)
		}

		displayName := ag.Name
		if ag.Emoji != "" {
			displayName = ag.Emoji + " " + displayName
		}
		fmt.Printf("Name:      %s\n", displayName)
		if ag.Description != "" {
			fmt.Printf("Role:      %s\n", ag.Description)
		}
		if ag.Voice != "" {
			fmt.Printf("Voice:     %s\n", ag.Voice)
		}
		fmt.Printf("Created:   %s\n", ag.CreatedAt.Format("2006-01-02 15:04:05"))

		return nil
	},
}

var agentModifyCmd = &cobra.Command{
	Use:   "modify <name> [field:value...]",
	Short: "Modify agent fields",
	Long: `Modify agent fields.

Examples:
  ttal agent modify yuki voice:af_heart
  ttal agent modify yuki emoji:🐱 description:'Task orchestration'`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])
		fieldUpdates, err := parseModifyArgs(args[1:])
		if err != nil {
			return err
		}

		if len(fieldUpdates) == 0 {
			return fmt.Errorf("no modifications specified (use field:value to update)")
		}

		ag, err := database.Agent.Query().
			Where(agent.Name(name)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("agent '%s' not found", name)
			}
			return fmt.Errorf("failed to get agent: %w", err)
		}

		updater := ag.Update()

		for field, value := range fieldUpdates {
			switch field {
			case "voice":
				if !voice.IsValidVoice(value) {
					return fmt.Errorf("unknown voice '%s' — run 'ttal voice list' to see available voices", value)
				}
				updater = updater.SetVoice(value)
			case "emoji":
				updater = updater.SetEmoji(value)
			case "description":
				updater = updater.SetDescription(value)
			default:
				return fmt.Errorf("unknown field '%s' (available: voice, emoji, description)", field)
			}
		}

		_, err = updater.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to modify agent: %w", err)
		}

		fmt.Printf("Agent '%s' updated successfully\n", name)
		fmt.Println("Fields updated:")
		for field, value := range fieldUpdates {
			fmt.Printf("  %s: %s\n", field, value)
		}

		return nil
	},
}

var agentDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Permanently delete an agent",
	Args:  cobra.ExactArgs(1),
	RunE:  runAgentDelete,
}

func runAgentDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	name := strings.ToLower(args[0])
	return deleteEntity("agent", name,
		func() (bool, error) { return database.Agent.Query().Where(agent.Name(name)).Exist(ctx) },
		func() (int, error) { return database.Agent.Delete().Where(agent.Name(name)).Exec(ctx) },
	)
}

func init() {
	rootCmd.AddCommand(agentCmd)

	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentModifyCmd)
	agentCmd.AddCommand(agentDeleteCmd)

	agentAddCmd.Flags().StringVar(&agentVoice, "voice", "", "Kokoro TTS voice ID (e.g. af_heart, af_sky)")
	agentAddCmd.Flags().StringVar(&agentEmoji, "emoji", "", "Display emoji (e.g. 🐱, 🦅)")
	agentAddCmd.Flags().StringVar(&agentDescription, "description", "", "Short role summary")
}

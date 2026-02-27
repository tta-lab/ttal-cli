package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/ent/agent"
	"codeberg.org/clawteam/ttal-cli/ent/project"
	"codeberg.org/clawteam/ttal-cli/ent/tag"
	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"codeberg.org/clawteam/ttal-cli/internal/voice"
	"github.com/spf13/cobra"
)

var (
	agentName        string
	agentVoice       string
	agentEmoji       string
	agentDescription string
	agentModel       string
	agentRuntime     string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Manage agents",
	Long:  `Add, list, get, and modify agents with tag-based filtering.`,
}

var agentAddCmd = &cobra.Command{
	Use:   "add <name> [+tag1 +tag2...]",
	Short: "Add a new agent",
	Long: `Add a new agent to the database.

Example:
  ttal agent add yuki +secretary +core`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])

		// Parse tags from args
		tagNames := parseTagsFromArgs(args[1:])

		// Create or get tags
		var tags []*ent.Tag
		for _, tagName := range tagNames {
			t, err := database.Tag.Query().
				Where(tag.Name(tagName)).
				Only(ctx)
			if ent.IsNotFound(err) {
				// Create tag if it doesn't exist
				t, err = database.Tag.Create().
					SetName(tagName).
					Save(ctx)
				if err != nil {
					return fmt.Errorf("failed to create tag %s: %w", tagName, err)
				}
			} else if err != nil {
				return fmt.Errorf("failed to query tag %s: %w", tagName, err)
			}
			tags = append(tags, t)
		}

		// Create agent with tags
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
		if agentModel != "" {
			if err := validateModel(agentModel); err != nil {
				return err
			}
			creator = creator.SetModel(agent.Model(agentModel))
		}
		if agentRuntime != "" {
			if err := runtime.Validate(agentRuntime); err != nil {
				return err
			}
			creator = creator.SetRuntime(agent.Runtime(agentRuntime))
		}
		if len(tags) > 0 {
			creator = creator.AddTags(tags...)
		}

		_, err := creator.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		fmt.Printf("Agent '%s' created successfully\n", name)
		if len(tagNames) > 0 {
			fmt.Printf("Tags: %s\n", formatTags(tagNames))
		}
		return nil
	},
}

var agentListCmd = &cobra.Command{
	Use:   "list [+tag1 +tag2...]",
	Short: "List agents",
	Long: `List all agents, optionally filtered by tags.

Examples:
  ttal agent list                    # List all agents
  ttal agent list +research          # Only agents with research tag`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Parse tags from args
		tagNames := parseTagsFromArgs(args)

		// Build query
		query := database.Agent.Query().WithTags()

		// Filter by tags if specified
		if len(tagNames) > 0 {
			query = query.Where(
				agent.HasTagsWith(tag.NameIn(tagNames...)),
			)
		}

		agents, err := query.All(ctx)
		if err != nil {
			return fmt.Errorf("failed to list agents: %w", err)
		}

		if len(agents) == 0 {
			fmt.Println("No agents found")
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tMODEL\tRUNTIME\tTAGS")
		for _, a := range agents {
			// Extract tag names from edges
			var tags []string
			for _, t := range a.Edges.Tags {
				tags = append(tags, t.Name)
			}

			name := a.Name
			if a.Emoji != "" {
				name = a.Emoji + " " + a.Name
			}

			model := ""
			if a.Model != "" {
				model = string(a.Model)
			}

			rt := string(runtime.ClaudeCode)
			if a.Runtime != nil {
				rt = string(*a.Runtime)
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				name, model, rt, formatTags(tags))
		}
		_ = w.Flush()

		return nil
	},
}

var agentInfoCmd = &cobra.Command{
	Use:   "info <name>",
	Short: "Get agent details",
	Long: `Get detailed information about an agent, including matching projects.

Example:
  ttal agent info yuki`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])

		// Get agent with tags
		ag, err := database.Agent.Query().
			Where(agent.Name(name)).
			WithTags().
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
		if ag.Model != "" {
			fmt.Printf("Model:     %s\n", ag.Model)
		}
		if ag.Voice != "" {
			fmt.Printf("Voice:     %s\n", ag.Voice)
		}
		if ag.Runtime != nil {
			fmt.Printf("Runtime:   %s\n", *ag.Runtime)
		}

		// Extract tag names
		var tagNames []string
		for _, t := range ag.Edges.Tags {
			tagNames = append(tagNames, t.Name)
		}
		if len(tagNames) > 0 {
			fmt.Printf("Tags:      %s\n", formatTags(tagNames))
		}

		fmt.Printf("Created:   %s\n", ag.CreatedAt.Format("2006-01-02 15:04:05"))

		// Show matching projects (projects that share at least one tag)
		if len(tagNames) > 0 {
			projects, err := database.Project.Query().
				Where(
					project.ArchivedAtIsNil(),
					project.HasTagsWith(tag.NameIn(tagNames...)),
				).
				All(ctx)
			if err != nil {
				return fmt.Errorf("failed to find matching projects: %w", err)
			}

			if len(projects) > 0 {
				fmt.Printf("\nMatching Projects:\n")
				for _, p := range projects {
					fmt.Printf("  - %s (%s)\n", p.Alias, p.Name)
				}
			}
		}

		return nil
	},
}

var agentModifyCmd = &cobra.Command{
	Use:   "modify <name> [+tag1 -tag2 field:value...]",
	Short: "Modify agent tags and fields",
	Long: `Add or remove tags and modify fields (taskwarrior-like syntax).

Examples:
  # Tag operations
  ttal agent modify yuki +secretary +core
  ttal agent modify yuki -old +research

  # Field modifications
  ttal agent modify yuki voice:af_heart

  # Combined operations
  ttal agent modify yuki emoji:🐱 +backend -legacy`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])
		addTagNames, removeTagNames, fieldUpdates := parseModifyArgs(args[1:])

		if len(addTagNames) == 0 && len(removeTagNames) == 0 && len(fieldUpdates) == 0 {
			return fmt.Errorf("no modifications specified (use +tag to add, -tag to remove, field:value to update)")
		}

		// Get agent
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

		// Apply field updates
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
			case "model":
				if err := validateModel(value); err != nil {
					return err
				}
				updater = updater.SetModel(agent.Model(value))
			case "runtime":
				if err := runtime.Validate(value); err != nil {
					return err
				}
				updater = updater.SetRuntime(agent.Runtime(value))
			default:
				return fmt.Errorf("unknown field '%s' (available: voice, emoji, description, model, runtime)", field)
			}
		}

		// Add tags
		if len(addTagNames) > 0 {
			for _, tagName := range addTagNames {
				t, err := database.Tag.Query().
					Where(tag.Name(tagName)).
					Only(ctx)
				if ent.IsNotFound(err) {
					// Create tag if it doesn't exist
					t, err = database.Tag.Create().
						SetName(tagName).
						Save(ctx)
					if err != nil {
						return fmt.Errorf("failed to create tag %s: %w", tagName, err)
					}
				} else if err != nil {
					return fmt.Errorf("failed to query tag %s: %w", tagName, err)
				}
				updater = updater.AddTags(t)
			}
		}

		// Remove tags
		if len(removeTagNames) > 0 {
			tagsToRemove, err := database.Tag.Query().
				Where(tag.NameIn(removeTagNames...)).
				All(ctx)
			if err != nil {
				return fmt.Errorf("failed to query tags to remove: %w", err)
			}
			updater = updater.RemoveTags(tagsToRemove...)
		}

		_, err = updater.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to modify agent: %w", err)
		}

		fmt.Printf("Agent '%s' updated successfully\n", name)
		if len(fieldUpdates) > 0 {
			fmt.Println("Fields updated:")
			for field, value := range fieldUpdates {
				fmt.Printf("  %s: %s\n", field, value)
			}
		}
		if len(addTagNames) > 0 {
			fmt.Printf("Tags added: %s\n", formatTags(addTagNames))
		}
		if len(removeTagNames) > 0 {
			fmt.Printf("Tags removed: %s\n", formatTags(removeTagNames))
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

var agentSyncTokensCmd = &cobra.Command{
	Use:   "sync-tokens",
	Short: "Fill bot tokens from env vars into config.toml",
	Long: `Read {AGENT}_BOT_TOKEN env vars for all agents in the database
and write the tokens into config.toml. New agents are added automatically.

Example:
  export KESTREL_BOT_TOKEN="123:ABC..."
  export YUKI_BOT_TOKEN="456:DEF..."
  ttal agent sync-tokens`,
	RunE: func(cmd *cobra.Command, args []string) error {
		agents, err := database.Agent.Query().All(cmd.Context())
		if err != nil {
			return fmt.Errorf("query agents: %w", err)
		}

		names := make([]string, len(agents))
		for i, a := range agents {
			names[i] = a.Name
		}

		return config.SyncTokens(names)
	},
}

// validateModel checks that the model value is one of the allowed options.
func validateModel(m string) error {
	switch m {
	case "haiku", "sonnet", "opus":
		return nil
	default:
		return fmt.Errorf("unknown model '%s' (available: haiku, sonnet, opus)", m)
	}
}

func init() {
	rootCmd.AddCommand(agentCmd)

	// Add subcommands
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentModifyCmd)
	agentCmd.AddCommand(agentDeleteCmd)
	agentCmd.AddCommand(agentSyncTokensCmd)

	// Flags for agent add
	agentAddCmd.Flags().StringVar(&agentVoice, "voice", "", "Kokoro TTS voice ID (e.g. af_heart, af_sky)")
	agentAddCmd.Flags().StringVar(&agentEmoji, "emoji", "", "Display emoji (e.g. 🐱, 🦅)")
	agentAddCmd.Flags().StringVar(&agentDescription, "description", "", "Short role summary")
	agentAddCmd.Flags().StringVar(&agentModel, "model", "", "Claude model tier (haiku, sonnet, opus)")
	agentAddCmd.Flags().StringVar(&agentRuntime, "runtime", "", "Coding agent runtime (claude-code, opencode)")
}

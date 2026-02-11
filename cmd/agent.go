package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/guion-opensource/ttal-cli/ent"
	"github.com/guion-opensource/ttal-cli/ent/agent"
	"github.com/guion-opensource/ttal-cli/ent/project"
	"github.com/guion-opensource/ttal-cli/ent/tag"
	"github.com/spf13/cobra"
)

var (
	agentName string
	agentPath string
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
  ttal agent add yuki --path=/Users/neil/clawd/.openclaw-main +secretary +core`,
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

		if agentPath != "" {
			creator = creator.SetPath(agentPath)
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
		_, _ = fmt.Fprintln(w, "NAME\tTAGS\tPATH")
		for _, a := range agents {
			// Extract tag names from edges
			var tags []string
			for _, t := range a.Edges.Tags {
				tags = append(tags, t.Name)
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
				a.Name, formatTags(tags), a.Path)
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

		fmt.Printf("Name:      %s\n", ag.Name)
		if ag.Path != "" {
			fmt.Printf("Path:      %s\n", ag.Path)
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
	Use:   "modify <name> [+tag1 -tag2...]",
	Short: "Modify agent tags",
	Long: `Add or remove tags from an agent (taskwarrior-like syntax).

Examples:
  ttal agent modify yuki +secretary +core
  ttal agent modify yuki -old +research
  ttal agent modify athena +backend +api -legacy`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		name := strings.ToLower(args[0])
		addTagNames, removeTagNames := parseModifyTags(args[1:])

		if len(addTagNames) == 0 && len(removeTagNames) == 0 {
			return fmt.Errorf("no tags to add or remove (use +tag to add, -tag to remove)")
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
			return fmt.Errorf("failed to modify tags: %w", err)
		}

		fmt.Printf("Agent '%s' tags updated\n", name)
		if len(addTagNames) > 0 {
			fmt.Printf("Added: %s\n", formatTags(addTagNames))
		}
		if len(removeTagNames) > 0 {
			fmt.Printf("Removed: %s\n", formatTags(removeTagNames))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(agentCmd)

	// Add subcommands
	agentCmd.AddCommand(agentAddCmd)
	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentInfoCmd)
	agentCmd.AddCommand(agentModifyCmd)

	// Flags for agent add
	agentAddCmd.Flags().StringVar(&agentPath, "path", "", "Agent workspace path")
}

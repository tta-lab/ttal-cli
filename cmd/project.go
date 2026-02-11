package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/guion-opensource/ttal-cli/ent"
	"github.com/guion-opensource/ttal-cli/ent/project"
	"github.com/guion-opensource/ttal-cli/ent/tag"
	"github.com/spf13/cobra"
)

var (
	projectAlias       string
	projectName        string
	projectDescription string
	projectPath        string
	projectRepo        string
	projectRepoType    string
	projectOwner       string
	includeArchived    bool
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Add, list, get, archive, and modify projects with tag-based filtering.`,
}

var projectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new project",
	Long: `Add a new project to the database.

Example:
  ttal project add --alias=clawd --name='TTAL Core' --path=/Users/neil/clawd --repo=neil/clawd --repo-type=forgejo`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if projectAlias == "" {
			return fmt.Errorf("--alias is required")
		}
		if projectName == "" {
			return fmt.Errorf("--name is required")
		}

		ctx := context.Background()

		// Parse tags from args (e.g., +backend +infrastructure)
		tagNames := parseTagsFromArgs(args)

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

		// Create project with tags
		creator := database.Project.Create().
			SetAlias(projectAlias).
			SetName(projectName)

		if projectDescription != "" {
			creator = creator.SetDescription(projectDescription)
		}
		if projectPath != "" {
			creator = creator.SetPath(projectPath)
		}
		if projectRepo != "" {
			creator = creator.SetRepo(projectRepo)
		}
		if projectRepoType != "" {
			creator = creator.SetRepoType(project.RepoType(projectRepoType))
		}
		if projectOwner != "" {
			creator = creator.SetOwner(projectOwner)
		}
		if len(tags) > 0 {
			creator = creator.AddTags(tags...)
		}

		_, err := creator.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		fmt.Printf("Project '%s' created successfully\n", projectAlias)
		if len(tagNames) > 0 {
			fmt.Printf("Tags: %s\n", formatTags(tagNames))
		}
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list [+tag1 +tag2...]",
	Short: "List projects",
	Long: `List all projects, optionally filtered by tags.

Examples:
  ttal project list                    # List all active projects
  ttal project list --archived         # Include archived projects
  ttal project list +backend           # Only projects with backend tag
  ttal project list +backend +core     # Projects with both tags`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		// Parse tags from args
		tagNames := parseTagsFromArgs(args)

		// Build query
		query := database.Project.Query().WithTags()

		// Filter by tags if specified
		if len(tagNames) > 0 {
			query = query.Where(
				project.HasTagsWith(tag.NameIn(tagNames...)),
			)
		}

		// Filter archived unless explicitly included
		if !includeArchived {
			query = query.Where(project.ArchivedAtIsNil())
		}

		projects, err := query.All(ctx)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if len(projects) == 0 {
			fmt.Println("No projects found")
			return nil
		}

		// Print table
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ALIAS\tNAME\tOWNER\tREPO\tTAGS\tSTATUS")
		for _, p := range projects {
			status := "active"
			if p.ArchivedAt != nil {
				status = "archived"
			}

			// Extract tag names from edges
			var tags []string
			for _, t := range p.Edges.Tags {
				tags = append(tags, t.Name)
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				p.Alias, p.Name, p.Owner, p.Repo, formatTags(tags), status)
		}
		_ = w.Flush()

		return nil
	},
}

var projectGetCmd = &cobra.Command{
	Use:   "get <alias>",
	Short: "Get project details",
	Long: `Get detailed information about a project.

Example:
  ttal project get clawd`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		alias := args[0]

		proj, err := database.Project.Query().
			Where(project.Alias(alias)).
			WithTags().
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("project '%s' not found", alias)
			}
			return fmt.Errorf("failed to get project: %w", err)
		}

		fmt.Printf("Alias:       %s\n", proj.Alias)
		fmt.Printf("Name:        %s\n", proj.Name)
		if proj.Description != "" {
			fmt.Printf("Description: %s\n", proj.Description)
		}
		if proj.Path != "" {
			fmt.Printf("Path:        %s\n", proj.Path)
		}
		if proj.Repo != "" {
			fmt.Printf("Repo:        %s\n", proj.Repo)
		}
		if proj.RepoType != "" {
			fmt.Printf("Repo Type:   %s\n", proj.RepoType)
		}
		if proj.Owner != "" {
			fmt.Printf("Owner:       %s\n", proj.Owner)
		}

		// Extract tag names
		var tagNames []string
		for _, t := range proj.Edges.Tags {
			tagNames = append(tagNames, t.Name)
		}
		if len(tagNames) > 0 {
			fmt.Printf("Tags:        %s\n", formatTags(tagNames))
		}

		fmt.Printf("Created:     %s\n", proj.CreatedAt.Format("2006-01-02 15:04:05"))
		if proj.ArchivedAt != nil {
			fmt.Printf("Archived:    %s\n", proj.ArchivedAt.Format("2006-01-02 15:04:05"))
		}

		return nil
	},
}

var projectArchiveCmd = &cobra.Command{
	Use:   "archive <alias>",
	Short: "Archive a project",
	Long: `Mark a project as archived.

Example:
  ttal project archive old-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		alias := args[0]

		now := time.Now()
		_, err := database.Project.Update().
			Where(project.Alias(alias)).
			SetArchivedAt(now).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to archive project: %w", err)
		}

		fmt.Printf("Project '%s' archived successfully\n", alias)
		return nil
	},
}

var projectUnarchiveCmd = &cobra.Command{
	Use:   "unarchive <alias>",
	Short: "Unarchive a project",
	Long: `Remove archived status from a project.

Example:
  ttal project unarchive old-project`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		alias := args[0]

		_, err := database.Project.Update().
			Where(project.Alias(alias)).
			ClearArchivedAt().
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to unarchive project: %w", err)
		}

		fmt.Printf("Project '%s' unarchived successfully\n", alias)
		return nil
	},
}

var projectModifyCmd = &cobra.Command{
	Use:   "modify <alias> [+tag1 -tag2 +tag3...]",
	Short: "Modify project tags",
	Long: `Add or remove tags from a project (taskwarrior-like syntax).

Examples:
  ttal project modify clawd +infrastructure +core
  ttal project modify clawd -old +new
  ttal project modify flicknote +backend +api -legacy`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		alias := args[0]
		addTagNames, removeTagNames := parseModifyTags(args[1:])

		if len(addTagNames) == 0 && len(removeTagNames) == 0 {
			return fmt.Errorf("no tags to add or remove (use +tag to add, -tag to remove)")
		}

		// Get project
		proj, err := database.Project.Query().
			Where(project.Alias(alias)).
			Only(ctx)
		if err != nil {
			if ent.IsNotFound(err) {
				return fmt.Errorf("project '%s' not found", alias)
			}
			return fmt.Errorf("failed to get project: %w", err)
		}

		updater := proj.Update()

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

		fmt.Printf("Project '%s' tags updated\n", alias)
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
	rootCmd.AddCommand(projectCmd)

	// Add subcommands
	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
	projectCmd.AddCommand(projectModifyCmd)

	// Flags for project add
	projectAddCmd.Flags().StringVar(&projectAlias, "alias", "", "Project alias (required, unique identifier)")
	projectAddCmd.Flags().StringVar(&projectName, "name", "", "Project name (required)")
	projectAddCmd.Flags().StringVar(&projectDescription, "description", "", "Project description")
	projectAddCmd.Flags().StringVar(&projectPath, "path", "", "Filesystem path")
	projectAddCmd.Flags().StringVar(&projectRepo, "repo", "", "Repository ID (e.g., neil/clawd)")
	projectAddCmd.Flags().StringVar(&projectRepoType, "repo-type", "", "Repository type (forgejo, github, codeberg)")
	projectAddCmd.Flags().StringVar(&projectOwner, "owner", "", "Project owner")

	// Flags for project list
	projectListCmd.Flags().BoolVar(&includeArchived, "archived", false, "Include archived projects")
}

// Helper functions

func parseTagsFromArgs(args []string) []string {
	var tags []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") {
			tagName := strings.TrimPrefix(arg, "+")
			tags = append(tags, strings.ToLower(tagName))
		}
	}
	return tags
}

func parseModifyTags(args []string) (addTags, removeTags []string) {
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") {
			tagName := strings.TrimPrefix(arg, "+")
			addTags = append(addTags, strings.ToLower(tagName))
		} else if strings.HasPrefix(arg, "-") {
			tagName := strings.TrimPrefix(arg, "-")
			removeTags = append(removeTags, strings.ToLower(tagName))
		}
	}
	return
}

func formatTags(tags []string) string {
	if len(tags) == 0 {
		return "-"
	}
	formatted := make([]string, 0, len(tags))
	for _, t := range tags {
		formatted = append(formatted, "+"+t)
	}
	return strings.Join(formatted, " ")
}

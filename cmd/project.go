package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/ent/project"
	"github.com/spf13/cobra"
)

var (
	projectAlias       string
	projectName        string
	projectDescription string
	projectPath        string
	archivedOnly       bool
)

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Add, list, get, archive, and modify projects.`,
}

var projectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new project",
	Long: `Add a new project to the database.

Example:
  ttal project add --alias=clawd --name='TTAL Core' --path=/Users/neil/clawd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if projectAlias == "" {
			return fmt.Errorf("--alias is required")
		}
		if projectName == "" {
			return fmt.Errorf("--name is required")
		}

		ctx := context.Background()

		creator := database.Project.Create().
			SetAlias(projectAlias).
			SetName(projectName)

		if projectDescription != "" {
			creator = creator.SetDescription(projectDescription)
		}
		if projectPath != "" {
			creator = creator.SetPath(projectPath)
		}
		_, err := creator.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		fmt.Printf("Project '%s' created successfully\n", projectAlias)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List projects",
	Long: `List all projects.

Examples:
  ttal project list                    # List all active projects
  ttal project list --archived         # List only archived projects`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		query := database.Project.Query()

		if archivedOnly {
			query = query.Where(project.ArchivedAtNotNil())
		} else {
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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "ALIAS\tNAME\tSTATUS")
		for _, p := range projects {
			status := "active"
			if p.ArchivedAt != nil {
				status = "archived"
			}

			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n",
				p.Alias, p.Name, status)
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

var projectDeleteCmd = &cobra.Command{
	Use:   "delete <alias>",
	Short: "Permanently delete a project",
	Args:  cobra.ExactArgs(1),
	RunE:  runProjectDelete,
}

func runProjectDelete(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	alias := strings.ToLower(args[0])
	return deleteEntity("project", alias,
		func() (bool, error) { return database.Project.Query().Where(project.Alias(alias)).Exist(ctx) },
		func() (int, error) { return database.Project.Delete().Where(project.Alias(alias)).Exec(ctx) },
	)
}

var projectModifyCmd = &cobra.Command{
	Use:   "modify <alias> [field:value...]",
	Short: "Modify project fields",
	Long: `Modify project fields.

Examples:
  ttal project modify clawd alias:new-alias
  ttal project modify clawd name:'New Project Name'
  ttal project modify clawd path:/new/path`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		alias := args[0]
		fieldUpdates, err := parseModifyArgs(args[1:])
		if err != nil {
			return err
		}

		if len(fieldUpdates) == 0 {
			return fmt.Errorf("no modifications specified (use field:value to update)")
		}

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

		for field, value := range fieldUpdates {
			switch field {
			case "alias":
				updater = updater.SetAlias(strings.ToLower(value))
			case "name":
				updater = updater.SetName(value)
			case "description":
				updater = updater.SetDescription(value)
			case "path":
				updater = updater.SetPath(value)
			default:
				return fmt.Errorf("unknown field '%s' (available: alias, name, description, path)", field)
			}
		}

		_, err = updater.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to modify project: %w", err)
		}

		fmt.Printf("Project '%s' updated successfully\n", alias)
		fmt.Println("Fields updated:")
		for field, value := range fieldUpdates {
			fmt.Printf("  %s: %s\n", field, value)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectGetCmd)
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectModifyCmd)

	projectAddCmd.Flags().StringVar(&projectAlias, "alias", "", "Project alias (required, unique identifier)")
	projectAddCmd.Flags().StringVar(&projectName, "name", "", "Project name (required)")
	projectAddCmd.Flags().StringVar(&projectDescription, "description", "", "Project description")
	projectAddCmd.Flags().StringVar(&projectPath, "path", "", "Filesystem path")
	projectListCmd.Flags().BoolVar(&archivedOnly, "archived", false, "Show only archived projects")
}

func parseModifyArgs(args []string) (fieldUpdates map[string]string, err error) {
	fieldUpdates = make(map[string]string)
	for _, arg := range args {
		if strings.HasPrefix(arg, "+") || strings.HasPrefix(arg, "-") {
			return nil, fmt.Errorf("tag operations (+tag/-tag) are no longer supported; tags are managed in config file")
		} else if strings.Contains(arg, ":") {
			parts := strings.SplitN(arg, ":", 2)
			if len(parts) == 2 {
				field := strings.ToLower(strings.TrimSpace(parts[0]))
				value := strings.TrimSpace(parts[1])
				fieldUpdates[field] = value
			}
		}
	}
	return
}

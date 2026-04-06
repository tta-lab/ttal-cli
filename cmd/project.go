package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/open"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const statusCol = 3 // index of the STATUS column in project list table

const projectAddExample = `ttal project add --alias myproj --name "My Project" --path /path/to/repo`

var (
	projectAlias string
	projectName  string
	projectPath  string
	projectJSON  bool
	archivedOnly bool
)

func getProjectStore() *project.Store {
	return project.NewStore(config.ResolveProjectsPath())
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Manage projects",
	Long:  `Add, list, archive, and modify projects.`,
}

var projectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a new project",
	Long: `Add a new project.

Example:
  ttal project add --alias=clawd --name='TTAL Core' --path=/Users/neil/clawd`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if projectAlias == "" {
			return fmt.Errorf("--alias is required\n\n  Example: %s", projectAddExample)
		}
		if projectName == "" {
			return fmt.Errorf("--name is required\n\n  Example: %s", projectAddExample)
		}

		store := getProjectStore()
		if err := store.Add(projectAlias, projectName, projectPath); err != nil {
			return fmt.Errorf("failed to create project: %w", err)
		}

		fmt.Printf("Project '%s' created successfully\n", projectAlias)
		return nil
	},
}

var projectListCmd = &cobra.Command{
	Use:   "list [team]",
	Short: "List projects",
	Long: `List all projects. Optionally specify a team name to list that team's projects
instead of the current team.

Examples:
  ttal project list                    # List current team's projects
  ttal project list guion              # List guion team's projects
  ttal project list --archived         # List only archived projects`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var store *project.Store
		if len(args) == 1 {
			teamPath := config.ResolveProjectsPathForTeam(args[0])
			store = project.NewStore(teamPath)
		} else {
			store = getProjectStore()
		}

		projects, err := store.List(archivedOnly)
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if projectJSON {
			type projectJSON struct {
				Alias string `json:"alias"`
				Name  string `json:"name"`
				Path  string `json:"path"`
			}
			output := make([]projectJSON, 0, len(projects))
			for _, p := range projects {
				output = append(output, projectJSON{Alias: p.Alias, Name: p.Name, Path: p.Path})
			}
			data, err := json.Marshal(output)
			if err != nil {
				return fmt.Errorf("failed to marshal projects: %w", err)
			}
			fmt.Println(string(data))
			return nil
		}

		if len(projects) == 0 {
			fmt.Println("No projects found")
			return nil
		}

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

		rows := make([][]string, 0, len(projects))
		for _, p := range projects {
			status := "active"
			if p.Archived {
				status = "archived"
			}
			rows = append(rows, []string{
				p.Alias,
				p.Name,
				p.Path,
				status,
			})
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				if col == statusCol {
					return dimStyle
				}
				return cellStyle
			}).
			Headers("ALIAS", "NAME", "PATH", "STATUS").
			Rows(rows...)

		lipgloss.Println(t)
		fmt.Printf("\n%d %s\n", len(projects), format.Plural(len(projects), "project", "projects"))
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
		store := getProjectStore()
		if err := store.Archive(args[0]); err != nil {
			return fmt.Errorf("failed to archive project: %w", err)
		}

		fmt.Printf("Project '%s' archived successfully\n", args[0])
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
		store := getProjectStore()
		if err := store.Unarchive(args[0]); err != nil {
			return fmt.Errorf("failed to unarchive project: %w", err)
		}

		fmt.Printf("Project '%s' unarchived successfully\n", args[0])
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
	alias := strings.ToLower(args[0])
	store := getProjectStore()
	return deleteEntity("project", alias,
		func() (bool, error) { return store.Exists(alias) },
		func() error { return store.Delete(alias) },
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
		alias := args[0]
		fieldUpdates, err := parseModifyArgs(args[1:])
		if err != nil {
			return err
		}

		if len(fieldUpdates) == 0 {
			return fmt.Errorf("no modifications specified\n\n  Example: ttal project modify myproj name:\"New Name\" path:/new/path") //nolint:lll
		}

		store := getProjectStore()
		if err := store.Modify(alias, fieldUpdates); err != nil {
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

var resolveJSON bool

var projectResolveCmd = &cobra.Command{
	Use:   "resolve [path]",
	Short: "Resolve project alias from a filesystem path",
	Long: `Resolve the project alias for a given path (defaults to current directory).

Examples:
  ttal project resolve                       # resolve cwd
  ttal project resolve /path/to/repo         # resolve explicit path
  ttal project resolve --json                # output {"alias":"...","task_id":"..."}`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var workDir string
		if len(args) == 1 {
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("invalid path: %w", err)
			}
			workDir = abs
		} else {
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			workDir = cwd
		}

		alias := project.ResolveProjectAlias(workDir)

		if resolveJSON {
			taskID := ""
			if alias != "" {
				tasks, err := taskwarrior.ExportTasksByFilter("+ACTIVE", fmt.Sprintf("project:%s", alias))
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: failed to lookup active task: %v\n", err)
				} else if len(tasks) > 0 {
					taskID = tasks[0].HexID()
				}
			}
			out, err := json.Marshal(map[string]string{"alias": alias, "task_id": taskID})
			if err != nil {
				return fmt.Errorf("failed to marshal output: %w", err)
			}
			fmt.Println(string(out))
			return nil
		}

		if alias == "" {
			return fmt.Errorf("no project found for %s", workDir)
		}
		fmt.Println(alias)
		return nil
	},
}

var projectOpenCmd = &cobra.Command{
	Use:   "open <alias>",
	Short: "Open project repository in browser",
	Long: `Open the project's remote repository URL in the default browser.

Resolves the project path from projects.toml, reads the git remote origin URL,
and constructs the web URL (github.com for GitHub, FORGEJO_URL for Forgejo repos).

Example:
  ttal project open ttal`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		alias := args[0]

		projectPath, err := project.GetProjectPath(alias)
		if err != nil {
			return err
		}

		repoInfo, err := gitprovider.DetectProvider(projectPath)
		if err != nil {
			return fmt.Errorf("cannot determine repo from %s: %w", projectPath, err)
		}

		repoURL := repoInfo.WebURL()
		if err := open.Browser(repoURL); err != nil {
			return fmt.Errorf("open browser for %s: %w", repoURL, err)
		}
		fmt.Printf("Opening %s/%s: %s\n", repoInfo.Owner, repoInfo.Repo, repoURL)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)

	projectCmd.AddCommand(projectAddCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectArchiveCmd)
	projectCmd.AddCommand(projectUnarchiveCmd)
	projectCmd.AddCommand(projectDeleteCmd)
	projectCmd.AddCommand(projectModifyCmd)
	projectCmd.AddCommand(projectResolveCmd)
	projectCmd.AddCommand(projectOpenCmd)

	projectAddCmd.Flags().StringVar(&projectAlias, "alias", "", "Project alias (required, unique identifier)")
	projectAddCmd.Flags().StringVar(&projectName, "name", "", "Project name (required)")
	projectAddCmd.Flags().StringVar(&projectPath, "path", "", "Filesystem path")
	projectListCmd.Flags().BoolVar(&projectJSON, "json", false, "Output as JSON")
	projectListCmd.Flags().BoolVar(&archivedOnly, "archived", false, "Show only archived projects")
	projectResolveCmd.Flags().BoolVar(&resolveJSON, "json", false, "Output as JSON with alias and task_id")
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

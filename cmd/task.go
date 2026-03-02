package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

var (
	findCompleted bool
	executeDryRun bool
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Taskwarrior task utilities",
	// Skip database initialization — task commands use taskwarrior directly
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
}

var taskGetCmd = &cobra.Command{
	Use:   "get <uuid>",
	Short: "Get formatted task prompt",
	Long: `Export a taskwarrior task and format it as a rich prompt.

Includes description, annotations, and inlined referenced documentation.
Useful for piping to agents or debugging task content.

Accepts 8-char UUID prefixes or full UUIDs.

Examples:
  ttal task get abc12345
  ttal task get abc12345 | ttal send --to eve --stdin`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := taskwarrior.ValidateUUID(args[0]); err != nil {
			return err
		}
		task, err := taskwarrior.ExportTask(args[0])
		if err != nil {
			return err
		}
		fmt.Print(task.FormatPrompt())
		return nil
	},
}

var taskFindCmd = &cobra.Command{
	Use:   "find <keyword> [keyword...]",
	Short: "Find tasks by keywords (OR match)",
	Long: `Search task descriptions for any of the given keywords.

Multiple keywords use OR logic — a task matches if its description
contains any of the keywords (case-insensitive).

By default only shows pending tasks. Use --completed to show completed tasks instead.

Examples:
  ttal task find respawn
  ttal task find mistral k3s ttal
  ttal task find --completed "voice note"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		status := "pending"
		if findCompleted {
			status = "completed"
		}
		tasks, err := taskwarrior.FindTasks(args, status)
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Fprintf(cmd.ErrOrStderr(), "No %s tasks found matching: %s\n", status, strings.Join(args, " | "))
			return nil
		}

		sort.Slice(tasks, func(i, j int) bool {
			return tasks[i].ID < tasks[j].ID
		})

		dimColor := lipgloss.Color("241")
		headerStyle := lipgloss.NewStyle().Bold(true).Padding(0, 1)
		cellStyle := lipgloss.NewStyle().Padding(0, 1)
		dimStyle := cellStyle.Foreground(dimColor)

		var rows [][]string
		for _, t := range tasks {
			uuid := t.UUID
			if len(uuid) > 8 {
				uuid = uuid[:8]
			}
			rows = append(rows, []string{
				fmt.Sprintf("%d", t.ID),
				uuid,
				t.Status,
				t.Project,
				strings.Join(t.Tags, " "),
				t.Description,
			})
		}

		tbl := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				switch col {
				case 0, 1, 2:
					return dimStyle
				default:
					return cellStyle
				}
			}).
			Headers("ID", "UUID", "Status", "Project", "Tags", "Description").
			Rows(rows...)

		fmt.Println(tbl)
		plural := "tasks"
		if len(tasks) == 1 {
			plural = "task"
		}
		fmt.Printf("\n%d %s\n", len(tasks), plural)
		return nil
	},
}

func agentNotConfigured(field, team string) error {
	return fmt.Errorf(
		"%s not configured for team %s\n\n"+
			"Add to config.toml:\n  [teams.%s]\n  %s = \"<agent-name>\"",
		field, team, team, field)
}

var taskDesignCmd = &cobra.Command{
	Use:   "design <uuid>",
	Short: "Route task to design agent",
	Long:  `Send a task to the team's design agent (design_agent in config).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		agent := cfg.DesignAgent()
		if agent == "" {
			return agentNotConfigured("design_agent", cfg.TeamName())
		}
		uuid := args[0]
		prompt := cfg.RenderPrompt("design", uuid)
		return routeTaskToAgent(agent, uuid, "task design", prompt)
	},
}

var taskResearchCmd = &cobra.Command{
	Use:   "research <uuid>",
	Short: "Route task to research agent",
	Long:  `Send a task to the team's research agent (research_agent in config).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		agent := cfg.ResearchAgent()
		if agent == "" {
			return agentNotConfigured("research_agent", cfg.TeamName())
		}
		uuid := args[0]
		prompt := cfg.RenderPrompt("research", uuid)
		return routeTaskToAgent(agent, uuid, "task research", prompt)
	},
}

var taskTestCmd = &cobra.Command{
	Use:   "test <uuid>",
	Short: "Route task to test agent",
	Long:  `Send a task to the team's test agent (test_agent in config).`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		agent := cfg.TestAgent()
		if agent == "" {
			return agentNotConfigured("test_agent", cfg.TeamName())
		}
		uuid := args[0]
		prompt := cfg.RenderPrompt("test", uuid)
		return routeTaskToAgent(agent, uuid, "task test", prompt)
	},
}

var taskExecuteCmd = &cobra.Command{
	Use:   "execute <uuid>",
	Short: "Spawn a worker for a task",
	Long: `Spawn a worker to execute a task. Resolves runtime from task tags
or team's worker_runtime config. Creates a git worktree and tmux session.

Use --dry-run to preview what would happen without actually spawning.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return spawnWorkerForTask(args[0], executeDryRun)
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskFindCmd)
	taskCmd.AddCommand(taskDesignCmd)
	taskCmd.AddCommand(taskResearchCmd)
	taskCmd.AddCommand(taskTestCmd)
	taskCmd.AddCommand(taskExecuteCmd)

	taskFindCmd.Flags().BoolVar(&findCompleted, "completed", false, "Show completed tasks instead of pending")
	taskExecuteCmd.Flags().BoolVar(&executeDryRun, "dry-run", false, "Show what would happen without spawning")
}

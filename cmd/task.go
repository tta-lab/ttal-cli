package cmd

import (
	"fmt"
	"sort"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/spf13/cobra"
)

var findCompleted bool

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

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskFindCmd)

	taskFindCmd.Flags().BoolVar(&findCompleted, "completed", false, "Show completed tasks instead of pending")
}

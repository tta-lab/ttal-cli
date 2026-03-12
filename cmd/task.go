package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/format"
	projectPkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

const taskStatusCompleted = "completed"

var (
	findCompleted      bool
	executeDryRun      bool
	executeYes         bool
	taskAddProject     string
	taskAddTags        []string
	taskAddPriority    string
	taskAddAnnotations []string
)

var taskCmd = &cobra.Command{
	Use:   "task",
	Short: "Taskwarrior task utilities",
}

var taskGetCmd = &cobra.Command{
	Use:   "get [uuid]",
	Short: "Get formatted task prompt",
	Long: `Export a taskwarrior task and format it as a rich prompt.

When no UUID is given, reads from TTAL_JOB_ID environment variable
(automatically set in worker sessions).

Includes description, annotations, and inlined referenced documentation.
Useful for piping to agents or debugging task content.

Accepts 8-char UUID prefixes or full UUIDs.

Examples:
  ttal task get              # uses $TTAL_JOB_ID
  ttal task get abc12345
  ttal task get abc12345 | ttal send --to eve --stdin`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var uuid string
		if len(args) > 0 {
			uuid = args[0]
		} else {
			uuid = os.Getenv("TTAL_JOB_ID")
			if uuid == "" {
				return fmt.Errorf("no UUID given and TTAL_JOB_ID not set\n\n" +
					"Either provide a UUID or run from a worker session")
			}
		}
		if err := taskwarrior.ValidateUUID(uuid); err != nil {
			return err
		}
		task, err := taskwarrior.ExportTask(uuid)
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
			status = taskStatusCompleted
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

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

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

var taskExecuteCmd = &cobra.Command{
	Use:   "execute <uuid>",
	Short: "Spawn a worker for a task",
	Long: `Spawn a worker to execute a task. Resolves runtime from task tags
or team's worker_runtime config. Creates a git worktree and tmux session.

Use --yes to confirm and spawn. Without --yes, shows the resolved project path and exits non-zero.
Use --dry-run to preview what would happen without actually spawning.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return spawnWorkerForTask(args[0], executeDryRun, executeYes)
	},
}

var taskAddCmd = &cobra.Command{
	Use:   "add <description>",
	Short: "Create a task with project validation",
	Long: `Create a taskwarrior task with validated project assignment.

The --project flag is required and must match an existing ttal project alias.

Examples:
  ttal task add --project ttal "Implement new feature"
  ttal task add --project ttal "Fix bug" --tag bug --priority H
  ttal task add --project fb "Add endpoint" --tag feature --annotate "Plan: flicknote abc12345"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := projectPkg.ValidateProjectAlias(taskAddProject); err != nil {
			return err
		}

		usage.Log("task.add", taskAddProject)

		if taskAddPriority != "" {
			switch strings.ToUpper(taskAddPriority) {
			case "H", "M", "L":
				taskAddPriority = strings.ToUpper(taskAddPriority)
			default:
				return fmt.Errorf("invalid priority %q — use H, M, or L", taskAddPriority)
			}
		}

		description := args[0]

		var modifiers []string
		modifiers = append(modifiers, fmt.Sprintf("project:%s", taskAddProject))
		for _, tag := range taskAddTags {
			modifiers = append(modifiers, "+"+strings.TrimPrefix(tag, "+"))
		}
		if taskAddPriority != "" {
			modifiers = append(modifiers, fmt.Sprintf("priority:%s", taskAddPriority))
		}

		uuid, err := taskwarrior.AddTask(description, modifiers...)
		if err != nil {
			return err
		}

		for _, ann := range taskAddAnnotations {
			if err := taskwarrior.AnnotateTask(uuid, ann); err != nil {
				return fmt.Errorf("task created (%s) but annotation failed: %w", uuid[:8], err)
			}
		}

		short := uuid
		if len(short) > 8 {
			short = short[:8]
		}
		fmt.Printf("Created task %s\n", short)
		if len(taskAddAnnotations) > 0 {
			fmt.Printf("  %d annotation(s) added\n", len(taskAddAnnotations))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskFindCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskRouteCmd)
	taskCmd.AddCommand(taskExecuteCmd)

	taskFindCmd.Flags().BoolVar(&findCompleted, "completed", false, "Show completed tasks instead of pending")
	taskExecuteCmd.Flags().BoolVar(&executeDryRun, "dry-run", false, "Show what would happen without spawning")
	taskExecuteCmd.Flags().BoolVar(&executeYes, "yes", false, "Bypass confirmation and execute immediately")

	taskAddCmd.Flags().StringVar(&taskAddProject, "project", "", "Project alias (required, must exist in ttal)")
	_ = taskAddCmd.MarkFlagRequired("project")
	taskAddCmd.Flags().StringArrayVar(&taskAddTags, "tag", nil, "Add tag (repeatable, e.g. --tag bug --tag urgent)")
	taskAddCmd.Flags().StringVar(&taskAddPriority, "priority", "", "Task priority (H, M, or L)")
	taskAddCmd.Flags().StringArrayVar(&taskAddAnnotations, "annotate", nil, "Add annotation (repeatable)")

}

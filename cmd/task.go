package cmd

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	projectPkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/skill"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/today"
	"github.com/tta-lab/ttal-cli/internal/tui"
	"github.com/tta-lab/ttal-cli/internal/usage"
)

const taskStatusCompleted = "completed"

var (
	findCompleted      bool
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

If a UUID is provided as an argument, that task is used directly.
Otherwise the task is auto-resolved from the session context:
  - Worker sessions: TTAL_JOB_ID
  - Agent sessions: TTAL_AGENT_NAME → active task with matching tag

Includes description, annotations, and inlined referenced documentation.
Useful for piping to agents or debugging task content.

Examples:
  ttal task get abc12345    # specific task by UUID
  ttal task get             # auto-resolves from session context
  ttal task get | ttal send --to eve --stdin`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var uuid string
		if len(args) > 0 {
			uuid = args[0]
		} else {
			resolved, err := resolveCurrentTask()
			if err != nil {
				return fmt.Errorf("no UUID provided and auto-resolve failed: %w", err)
			}
			uuid = resolved
		}
		if err := taskwarrior.ValidateUUID(uuid); err != nil {
			return err
		}

		// Use ExportTaskByHexID for short hex prefixes (8 chars),
		// ExportTask for full UUIDs (36 chars with dashes).
		var task *taskwarrior.Task
		var err error
		if len(uuid) != 36 {
			task, err = taskwarrior.ExportTaskByHexID(uuid, "")
		} else {
			task, err = taskwarrior.ExportTask(uuid)
		}
		if err != nil {
			return err
		}

		if task.Project != "" {
			if proj := projectPkg.ResolveProject(task.Project); proj != nil {
				displayPath := proj.Path
				if cwd, err := os.Getwd(); err == nil {
					worktreesRoot := config.WorktreesRoot()
					if worktreesRoot != "" && strings.HasPrefix(cwd, worktreesRoot) {
						displayPath = cwd // show actual worktree path when inside one
					}
				}
				fmt.Printf("Project: %s — %s\nPath: %s\n\n", proj.Alias, proj.Name, displayPath)
			}
		}

		fmt.Print(task.FormatPrompt())

		// Show skills section and footnote for manager agent sessions.
		if os.Getenv("TTAL_AGENT_NAME") != "" && os.Getenv("TTAL_JOB_ID") == "" {
			if skills := buildSkillsSection(); skills != "" {
				fmt.Print(skills)
			}
			fmt.Print("\n---\nThis is the complete task context. Do not run task export, task info, or task " +
				"annotations — everything is already here. Start working.\n")
		}

		return nil
	},
}

// buildSkillsSection returns a markdown skills section for the current manager agent.
// Returns empty string if the agent has no skills or skills cannot be resolved.
func buildSkillsSection() string {
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		return ""
	}

	cfg, err := config.Load()
	if err != nil {
		return ""
	}
	teamPath := cfg.TeamPath
	if teamPath == "" {
		return ""
	}

	role, err := agentfs.RoleOf(teamPath, agentName)
	if err != nil || role == "" {
		return ""
	}

	rolesCfg, err := config.LoadRoles()
	if err != nil || rolesCfg == nil {
		return ""
	}

	skillsList := rolesCfg.RoleSkills(role)
	if len(skillsList) == 0 {
		return ""
	}

	skillsDir := skill.DefaultSkillsDir()
	var lines []string
	for _, name := range skillsList {
		s, err := skill.GetSkill(skillsDir, name)
		desc := ""
		if err == nil {
			desc = s.Description
		}
		if err != nil {
			lines = append(lines, fmt.Sprintf("- `ttal skill get %s` ⚠️ (unavailable: %v)", name, err))
		} else if desc != "" {
			lines = append(lines, fmt.Sprintf("- `ttal skill get %s` — %s. Agent fetches on demand.", name, desc))
		} else {
			lines = append(lines, fmt.Sprintf("- `ttal skill get %s`", name))
		}
	}

	if len(lines) == 0 {
		return ""
	}
	return "\n## Skills\n\n" + strings.Join(lines, "\n") + "\n"
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
			return tasks[i].Urgency > tasks[j].Urgency
		})

		dimColor, headerStyle, cellStyle, dimStyle := format.TableStyles()

		var rows [][]string
		for _, t := range tasks {
			rows = append(rows, []string{
				t.HexID(),
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
				case 0, 1:
					return dimStyle
				default:
					return cellStyle
				}
			}).
			Headers("ID", "Status", "Project", "Tags", "Description").
			Rows(rows...)

		lipgloss.Println(tbl)
		plural := "tasks"
		if len(tasks) == 1 {
			plural = "task" //nolint:goconst
		}
		fmt.Printf("\n%d %s\n", len(tasks), plural)
		return nil
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
				return fmt.Errorf("invalid priority %q — use H, M, or L\n\n  Example: ttal task add --project ttal \"description\" --priority M", taskAddPriority) //nolint:lll
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

var taskHeatmapCmd = &cobra.Command{
	Use:   "heatmap",
	Short: "Show task completion heatmap for the past year",
	Long: `Print a compact GitHub-style heatmap of completed tasks for the past year.

Example:
  ttal task heatmap`,
	RunE: func(cmd *cobra.Command, args []string) error {
		counts, err := today.CompletedCounts()
		if err != nil {
			return fmt.Errorf("loading completed tasks: %w", err)
		}
		lipgloss.Print(tui.RenderHeatmap(counts, time.Now()))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(taskCmd)
	taskCmd.AddCommand(taskGetCmd)
	taskCmd.AddCommand(taskFindCmd)
	taskCmd.AddCommand(taskAddCmd)
	taskCmd.AddCommand(taskHeatmapCmd)

	taskFindCmd.Flags().BoolVar(&findCompleted, "completed", false, "Show completed tasks instead of pending")

	taskAddCmd.Flags().StringVar(&taskAddProject, "project", "", "Project alias (required, must exist in ttal)")
	_ = taskAddCmd.MarkFlagRequired("project")
	taskAddCmd.Flags().StringArrayVar(&taskAddTags, "tag", nil, "Add tag (repeatable, e.g. --tag bug --tag urgent)")
	taskAddCmd.Flags().StringVar(&taskAddPriority, "priority", "", "Task priority (H, M, or L)")
	taskAddCmd.Flags().StringArrayVar(&taskAddAnnotations, "annotate", nil, "Add annotation (repeatable)")

}

package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/format"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	projectpkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/skill"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "Inspect pipeline definitions",
	Long:  `List and inspect pipeline definitions from pipelines.toml.`,
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all pipelines",
	Long: `List all pipeline definitions with their descriptions and matching tags.

Example:
  ttal pipeline list`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return fmt.Errorf("load pipelines: %w", err)
		}
		if len(cfg.Pipelines) == 0 {
			fmt.Println("No pipelines configured")
			return nil
		}

		dimColor, headerStyle, cellStyle, _ := format.TableStyles()

		names := cfg.SortedNames()
		rows := make([][]string, 0, len(names))
		for _, name := range names {
			p := cfg.Pipelines[name]
			rows = append(rows, []string{
				name,
				p.Description,
				strings.Join(p.Tags, ", "),
			})
		}

		t := table.New().
			Border(lipgloss.RoundedBorder()).
			BorderStyle(lipgloss.NewStyle().Foreground(dimColor)).
			StyleFunc(func(row, col int) lipgloss.Style {
				if row == table.HeaderRow {
					return headerStyle
				}
				return cellStyle
			}).
			Headers("NAME", "DESCRIPTION", "TAGS").
			Rows(rows...)

		lipgloss.Println(t)
		fmt.Printf("\n%d %s\n", len(names), format.Plural(len(names), "pipeline", "pipelines"))
		return nil
	},
}

var pipelineGetCmd = &cobra.Command{
	Use:   "get <name>",
	Short: "Show pipeline stages",
	Long: `Show the stages of a pipeline as a simple terminal graph.

Example:
  ttal pipeline get standard
  ttal pipeline get bugfix`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := pipeline.Load(config.DefaultConfigDir())
		if err != nil {
			return fmt.Errorf("load pipelines: %w", err)
		}

		name := args[0]
		p, ok := cfg.Pipelines[name]
		if !ok {
			return fmt.Errorf("pipeline %q not found\n\nAvailable: %s", name, strings.Join(cfg.SortedNames(), ", "))
		}

		fmt.Printf("%s — %s\n", name, p.Description)
		fmt.Printf("Tags: %s\n\n", strings.Join(p.Tags, ", "))
		renderPipelineGraph(p)
		return nil
	},
}

// renderPipelineGraph prints pipeline stages as a linear graph.
// Example output:
//
//	Plan [designer] ──human──▸ Implement [coder]
//
// Reviewer info is shown inline when present:
//
//	Plan [designer] ──human/plan-review-lead──▸ Implement [coder]
func renderPipelineGraph(p pipeline.Pipeline) {
	for i, s := range p.Stages {
		if i > 0 {
			// Print arrow from previous stage to this one.
			prev := p.Stages[i-1]
			label := prev.Gate
			if prev.Reviewer != "" {
				label += "/" + prev.Reviewer
			}
			fmt.Printf(" ──%s──▸ ", label)
		}
		fmt.Printf("%s [%s]", s.Name, s.Assignee)
	}
	fmt.Println()
}

// pipelinePromptCmd outputs the role-specific prompt for the current task and pipeline stage.
// It is called by the CC SessionStart hook (via the context template's $ ttal pipeline prompt line)
// and must produce empty output when no relevant task or stage is found.
var pipelinePromptCmd = &cobra.Command{
	Use:   "prompt",
	Short: "Output the role prompt for the current task's pipeline stage",
	Long: `Output the role-specific prompt for the current task's pipeline stage.

Called by the CC SessionStart hook via the context template. Reads TTAL_JOB_ID
(worker/reviewer path) or TTAL_AGENT_NAME (manager path) to find the current task.
Outputs the role prompt with skills prepended. Outputs nothing when no task or stage
is found — always exits 0 (non-zero exits would fail the context hook).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := resolvePipelinePrompt()
		if prompt != "" {
			fmt.Println(prompt)
		}
		return nil
	},
}

// resolvePipelinePrompt builds the pipeline prompt for the current session.
// Skills are always injected based on the agent's role (never by stage).
// Task-specific content is appended only when a task exists.
// Exits early with "" when no role prompt is configured (always exits 0 for CC hook).
//
//nolint:gocyclo
func resolvePipelinePrompt() string {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[pipeline prompt] config load failed: %v", err)
		return ""
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")

	// Always inject role-based skills — unconditional, not gated on task.
	var skillContent string
	var role string
	if agentName != "" {
		teamPath := cfg.TeamPath()
		if teamPath != "" {
			if resolvedRole, err := agentfs.RoleOf(teamPath, agentName); err == nil && resolvedRole != "" {
				role = resolvedRole
				if roles := cfg.Roles(); roles != nil {
					skills := roles.RoleSkills(role)
					skillContent = skill.FetchContents(skills)
				}
			}
		}
	}

	task := resolveCurrentTaskForPrompt()

	// No task — output role-based skills only with a base role prompt.
	if task == nil {
		// Use role as the prompt key (e.g. "fixer", "designer").
		// Fall back to agentName if role is empty (shouldn't happen for valid agents).
		// Reviewer status cannot be determined without a stage, so reviewers
		// without active tasks get their base role prompt.
		promptKey := role
		if promptKey == "" {
			promptKey = agentName
		}
		if promptKey == "" {
			promptKey = "default"
		}
		rolePrompt := cfg.Prompt(promptKey)
		if skillContent != "" {
			if rolePrompt != "" {
				return skillContent + "\n\n---\n\n" + rolePrompt
			}
			return skillContent
		}
		return rolePrompt
	}

	// Active task — use existing task-first pipeline path for accurate reviewer detection.
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		log.Printf("[pipeline prompt] pipeline load failed: %v", err)
		return skillContent
	}

	_, p, err := pipelineCfg.MatchPipeline(task.Tags)
	if err != nil {
		log.Printf("[pipeline prompt] pipeline match failed for task %s: %v", task.HexID(), err)
		return skillContent
	}
	if p == nil {
		return skillContent
	}

	_, stage, err := p.CurrentStage(task.Tags)
	if err != nil {
		log.Printf("[pipeline prompt] stage resolution failed for task %s: %v", task.HexID(), err)
		return skillContent
	}
	if stage == nil {
		return skillContent
	}

	promptKey := resolvePromptKey(stage)
	rolePrompt := cfg.Prompt(promptKey)
	if rolePrompt == "" {
		return skillContent
	}

	taskPrompt := formatTaskPromptForPipeline(task)

	var combined strings.Builder
	if skillContent != "" {
		combined.WriteString(skillContent)
	}
	if taskPrompt != "" {
		if combined.Len() > 0 {
			combined.WriteString("\n\n---\n\n## Task\n\n")
		} else {
			combined.WriteString("## Task\n\n")
		}
		combined.WriteString(taskPrompt)
	}
	if combined.Len() > 0 {
		combined.WriteString("\n\n---\n\n")
	}
	combined.WriteString(rolePrompt)

	return expandPromptVars(combined.String(), task, cfg)
}

// resolveCurrentTaskForPrompt finds the task for the current session via TTAL_JOB_ID or TTAL_AGENT_NAME.
// Returns nil when no task is found (non-fatal).
func resolveCurrentTaskForPrompt() *taskwarrior.Task {
	if hexID := os.Getenv("TTAL_JOB_ID"); hexID != "" {
		task, err := taskwarrior.ExportTaskByHexID(hexID, "")
		if err != nil {
			log.Printf("[pipeline prompt] task lookup by TTAL_JOB_ID=%s failed: %v", hexID, err)
			return nil
		}
		return task
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		return nil
	}

	cfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		log.Printf("[pipeline prompt] load pipeline config: %v", err)
		return nil
	}
	tasks, err := pipeline.ActiveTasksByOwner(cfg, agentName)
	if err != nil {
		log.Printf("[pipeline prompt] task lookup for TTAL_AGENT_NAME=%s failed: %v", agentName, err)
		return nil
	}
	if len(tasks) == 0 {
		return nil
	}
	return &tasks[0]
}

// resolvePromptKey determines which config prompt key to use for the given stage,
// taking the current agent identity (TTAL_AGENT_NAME) into account for reviewer sessions.
func resolvePromptKey(stage *pipeline.Stage) string {
	agentName := os.Getenv("TTAL_AGENT_NAME")

	// Reviewer path: agent is the stage's reviewer, not the assignee.
	if agentName != "" && stage.Reviewer == agentName {
		if stage.IsWorker() {
			return "review"
		}
		return "plan_review"
	}

	return stage.Assignee
}

// expandPromptVars expands task-specific template variables in the prompt,
// including {{task-id}}, {{pr-number}}, {{owner}}, {{repo}}, {{branch}}, and {{skill:name}}.
func expandPromptVars(prompt string, task *taskwarrior.Task, cfg *config.Config) string {
	rt := cfg.DefaultRuntime()

	// Expand PR vars — soft failure: use empty strings if resolution fails.
	if task.PRID != "" {
		prInfo, err := taskwarrior.ParsePRID(task.PRID)
		if err == nil {
			branch := worker.CurrentBranch(task.UUID, task.Project, "")
			owner, repo := resolvePROwnerRepo(task)
			replacer := strings.NewReplacer(
				"{{pr-number}}", fmt.Sprintf("%d", prInfo.Index),
				"{{pr-title}}", task.Description,
				"{{owner}}", owner,
				"{{repo}}", repo,
				"{{branch}}", branch,
			)
			prompt = replacer.Replace(prompt)
		}
	}

	return config.RenderTemplate(prompt, task.HexID(), rt)
}

// formatTaskPromptForPipeline formats a task into a pipeline prompt block.
// Resolves project path and uses task.FormatPrompt() for the task content.
// Returns empty string if task cannot be resolved (non-fatal).
func formatTaskPromptForPipeline(task *taskwarrior.Task) string {
	if task == nil {
		return ""
	}
	projPath := projectpkg.ResolveProjectPath(task.Project)
	proj := projectpkg.ResolveProject(task.Project)
	if proj != nil && projPath != "" {
		return fmt.Sprintf("Project: %s — %s\\nPath: %s\\n\\n%s",
			task.Project, proj.Name, projPath, task.FormatPrompt())
	}
	return task.FormatPrompt()
}

// resolvePROwnerRepo resolves the owner and repo for a task's project.
// Soft failure: returns empty strings if the project path or git remote cannot be resolved.
func resolvePROwnerRepo(task *taskwarrior.Task) (owner, repo string) {
	projectPath := projectpkg.ResolveProjectPath(task.Project)
	if projectPath == "" {
		return "", ""
	}
	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		log.Printf("[pipeline prompt] could not detect git provider for project %q"+
			" ({{owner}}/{{repo}} will be empty): %v", task.Project, err)
		return "", ""
	}
	return info.Owner, info.Repo
}

func init() {
	rootCmd.AddCommand(pipelineCmd)
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineGetCmd)
	pipelineCmd.AddCommand(pipelinePromptCmd)
}

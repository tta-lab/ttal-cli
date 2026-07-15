package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// resolveJSONOutput is the JSON payload returned by `ttal project resolve --json`.
type resolveJSONOutput struct {
	Alias  string `json:"alias"`
	Path   string `json:"path"`
	TaskID string `json:"task_id"`
	Stage  string `json:"stage"`
	Owner  string `json:"owner"`
}

func buildResolveJSONOutput(
	alias string,
	proj *project.Project,
	task *taskwarrior.Task,
	pipelineCfg *pipeline.Config,
) resolveJSONOutput {
	out := resolveJSONOutput{Alias: alias}
	if proj != nil {
		out.Path = proj.Path
	}
	if task != nil {
		out.TaskID = task.HexID()
		out.Owner = task.Owner
		if pipelineCfg != nil {
			if _, p, err := pipelineCfg.MatchPipeline(task.Tags); err == nil && p != nil {
				if _, stage, err := p.CurrentStage(task.Tags); err == nil && stage != nil {
					out.Stage = stage.Name
				}
			}
		}
	}
	return out
}

var projectCmd = &cobra.Command{
	Use:   "project",
	Short: "Provide TTAL-specific project context",
	Long:  `Use the project CLI to list and resolve projects. This command provides TTAL-specific task context.`,
	Args:  cobra.NoArgs,
}

var resolveJSON bool

var projectResolveCmd = &cobra.Command{
	Use:   "resolve --json [path]",
	Short: "Resolve project context with active task details",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !resolveJSON {
			return fmt.Errorf("use `project resolve [path]` for project resolution")
		}

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

		if alias == "" {
			payload := buildResolveJSONOutput("", nil, nil, nil)
			out, _ := json.Marshal(payload)
			fmt.Println(string(out))
			return nil
		}

		var proj *project.Project
		var task *taskwarrior.Task
		var pipelineCfg *pipeline.Config

		if p, err := project.Get(alias); err == nil && p != nil {
			proj = p
		} else if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to lookup project %q: %v\n", alias, err)
		}

		tasks, err := taskwarrior.ExportTasksByFilter("+ACTIVE", fmt.Sprintf("project:%s", alias))
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to lookup active task: %v\n", err)
		} else if len(tasks) > 0 {
			task = &tasks[0]
		}

		if cfg, err := pipeline.Load(config.DefaultConfigDir()); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to load pipelines.toml: %v\n", err)
		} else {
			pipelineCfg = cfg
		}

		payload := buildResolveJSONOutput(alias, proj, task, pipelineCfg)
		out, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal output: %w", err)
		}
		fmt.Println(string(out))
		return nil
	},
}

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectResolveCmd)
	projectResolveCmd.Flags().BoolVar(&resolveJSON, "json", false,
		"Output as JSON with alias, path, task_id, stage, and owner")
}

// parseModifyArgs parses field:value arguments for modify commands (used by agent.go).
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

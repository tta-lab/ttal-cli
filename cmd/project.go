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
	Short: "Resolve projects (read-only; writes via ~/.config/ttal/projects.toml directly)",
	Long:  `Resolve project aliases and paths via the project CLI. Writes go directly to projects.toml.`,
}

var projectListCmd = &cobra.Command{
	Use:   "list", //nolint:goconst
	Short: "List projects",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		projects, err := project.List()
		if err != nil {
			return fmt.Errorf("failed to list projects: %w", err)
		}

		if projectJSON {
			data, err := json.Marshal(projects)
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

		for _, p := range projects {
			org := projectOrg(p.Path)
			fmt.Printf("%-12s %-20s %-40s %s\n", p.Alias, org, p.Name, p.Path)
		}

		fmt.Printf("\n%d projects — use project list --json for full details\n", len(projects))
		return nil
	},
}

var resolveJSON bool

var projectResolveCmd = &cobra.Command{
	Use:   "resolve [path]",
	Short: "Resolve project alias from a filesystem path",
	Args:  cobra.MaximumNArgs(1),
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

		if alias == "" {
			if resolveJSON {
				payload := buildResolveJSONOutput("", nil, nil, nil)
				out, _ := json.Marshal(payload)
				fmt.Println(string(out))
				return nil
			}
			fmt.Println("")
			return nil
		}

		if resolveJSON {
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
		}

		fmt.Println(alias)
		return nil
	},
}

var projectJSON bool

func init() {
	rootCmd.AddCommand(projectCmd)
	projectCmd.AddCommand(projectListCmd)
	projectCmd.AddCommand(projectResolveCmd)
	projectListCmd.Flags().BoolVar(&projectJSON, "json", false, "Output as JSON")
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

// projectOrg derives an org from a project path for display.
func projectOrg(p string) string {
	for _, part := range []string{"/projects/", "/references/"} {
		if idx := strings.Index(strings.ToLower(p), part); idx >= 0 {
			p = p[idx+len(part):]
			parts := strings.SplitN(p, "/", 2)
			if len(parts[0]) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

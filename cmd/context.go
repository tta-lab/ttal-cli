package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/promptrender"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// defaultTeamName is the single, hardcoded team name.
const defaultTeamName = "default"

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Render the agent wake-orientation bundle",
	Long: `Render the agent wake-orientation bundle.

Picks the manager or worker template based on whether the agent has an AGENTS.md
under team_path, then renders the template — $ cmd lines are executed with agent
env vars. Outputs plain markdown to stdout.

Managers get: diary + agent list + project list + pairing + role prompt + task.
Workers get: pairing + role prompt + task.

Always exits 0.`,
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

func runContext(_ *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[context] config load failed: %v", err)
		return nil
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		log.Printf("[context] TTAL_AGENT_NAME not set — skipping context")
		return nil
	}

	isManager := agentfs.HasAgent(cfg.TeamPath, agentName)
	var tmpl string
	if isManager {
		tmpl = cfg.Prompt("context_manager")
	} else {
		tmpl = cfg.Prompt("context_worker")
	}
	if tmpl == "" {
		log.Printf("[context] no template found for agent=%s (manager=%v)", agentName, isManager)
		return nil
	}

	cwd, _ := os.Getwd()
	env := buildContextEnv(agentName, cwd)

	output := promptrender.RenderTemplate(tmpl, agentName, defaultTeamName, env)
	fmt.Print(output)
	return nil
}

// buildContextEnv constructs the env slice for subprocess execution in the context template.
// Workers (cwd under ~/.ttal/worktrees/) get TTAL_JOB_ID derived from the worktree dir name.
func buildContextEnv(agentName, cwd string) []string {
	env := []string{
		"TTAL_AGENT_NAME=" + agentName,
	}
	if hexID := worker.TaskHexFromCwd(cwd); hexID != "" {
		env = append(env, "TTAL_JOB_ID="+hexID)
	}
	return env
}

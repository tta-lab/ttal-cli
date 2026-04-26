package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/promptrender"
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
		return nil
	}

	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
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
	if hexID := extractWorktreeHexID(cwd); hexID != "" {
		env = append(env, "TTAL_JOB_ID="+hexID)
	}
	return env
}

// extractWorktreeHexID extracts the hex task ID from a worktree CWD path.
// Worktree dirs follow the pattern ~/.ttal/worktrees/<hexID>-<alias>.
func extractWorktreeHexID(cwd string) string {
	if cwd == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[context] could not resolve home dir: %v", err)
		return ""
	}
	worktreesRoot := filepath.Join(home, ".ttal", "worktrees")
	if !strings.HasPrefix(cwd, worktreesRoot+string(filepath.Separator)) {
		return ""
	}
	rel := strings.TrimPrefix(cwd, worktreesRoot+string(filepath.Separator))
	dirName := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	parts := strings.SplitN(dirName, "-", 2)
	return parts[0]
}

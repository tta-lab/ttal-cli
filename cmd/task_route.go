package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	gitutil "github.com/tta-lab/ttal-cli/internal/git"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var routeToAgent string

var taskRouteCmd = &cobra.Command{
	Use:   "route <uuid>",
	Short: "Route task to a specific agent",
	Long: `Route a task to a named agent. The agent's role (from CLAUDE.md frontmatter)
determines which prompt template is used.

Examples:
  ttal task route abc12345 --to inke
  ttal task route abc12345 --to athena`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		agent, err := agentfs.Get(cfg.TeamPath(), routeToAgent)
		if err != nil {
			return err
		}
		uuid := args[0]
		rt := cfg.AgentRuntimeFor(routeToAgent)
		role := agent.Role
		if role == "" {
			role = "default"
		}
		prompt := cfg.RenderPrompt(role, uuid, rt)
		if prompt == "" {
			return fmt.Errorf("no prompt for role %q, no [default] in roles.toml, and no fallback in config.toml", role)
		}
		return routeTaskToAgent(routeToAgent, uuid, "task "+role, prompt)
	},
}

func init() {
	taskRouteCmd.Flags().StringVar(&routeToAgent, "to", "", "Agent name to route to (required)")
	_ = taskRouteCmd.MarkFlagRequired("to")
}

// routeTaskToAgent sends a task assignment message to a named agent via the daemon.
func routeTaskToAgent(agentName, taskUUID, roleTag, rolePrompt string) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	uuid := task.UUID
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}

	msg := fmt.Sprintf("[%s] %s — %s\n%s",
		roleTag, uuid, task.Description, rolePrompt)

	if err := daemon.Send(daemon.SendRequest{
		To:      agentName,
		Message: msg,
	}); err != nil {
		return err
	}

	fmt.Printf("Routed task %s to %s\n", uuid, agentName)
	return nil
}

// spawnWorkerForTask spawns a worker for a task using the standard spawn flow.
// When dryRun is true, it prints what would happen without spawning.
func spawnWorkerForTask(taskUUID string, dryRun bool) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	if task.Status == "completed" {
		return fmt.Errorf("task %s is already completed — cannot execute", taskUUID[:8])
	}

	sessionName := task.SessionName()
	if tmux.SessionExists(sessionName) {
		return fmt.Errorf("session %s already exists — cannot spawn duplicate", sessionName)
	}

	if task.ProjectPath == "" {
		return fmt.Errorf(
			"task %s has no project_path — run enrichment first "+
				"(task add usually triggers this automatically)",
			taskUUID)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rt := cfg.WorkerRuntime()
	for _, t := range task.Tags {
		switch t {
		case string(runtime.OpenCode), "oc":
			rt = runtime.OpenCode
		case string(runtime.Codex), "cx":
			rt = runtime.Codex
		}
	}

	workerName := strings.TrimPrefix(task.Branch, "worker/")
	if workerName == "" {
		workerName = task.SessionName()
	}

	if dryRun {
		printDryRun(task, rt, workerName)
		return nil
	}

	if err := taskwarrior.StartTask(task.UUID); err != nil {
		if strings.Contains(err.Error(), "already active") {
			fmt.Fprintf(os.Stderr, "Warning: task is already active\n")
		} else {
			return fmt.Errorf("task start failed before worker spawn: %w", err)
		}
	}

	if err := worker.Spawn(worker.SpawnConfig{
		Name:     workerName,
		Project:  task.ProjectPath,
		TaskUUID: task.UUID,
		Worktree: true,
		Runtime:  rt,
		Spawner:  detectSpawner(),
	}); err != nil {
		return err
	}

	return nil
}

// detectSpawner returns the team:agent identity from env vars.
// The daemon sets TTAL_AGENT_NAME and TTAL_TEAM for every agent session.
// Returns empty string if not running inside an agent session.
func detectSpawner() string {
	agent := os.Getenv("TTAL_AGENT_NAME")
	if agent == "" {
		return ""
	}
	team := os.Getenv("TTAL_TEAM")
	if team == "" {
		team = "default"
	}
	return team + ":" + agent
}

func printDryRun(task *taskwarrior.Task, rt runtime.Runtime, workerName string) {
	fmt.Printf("Task:        %s\n", task.Description)
	fmt.Printf("UUID:        %s\n", task.UUID)
	fmt.Printf("Project:     %s\n", task.ProjectPath)

	if gitRoot, err := gitutil.FindRoot(task.ProjectPath); err == nil {
		resolvedProject, _ := filepath.EvalSymlinks(task.ProjectPath)
		resolvedRoot, _ := filepath.EvalSymlinks(gitRoot)
		if resolvedProject != resolvedRoot {
			if rel, err := filepath.Rel(gitRoot, task.ProjectPath); err == nil {
				fmt.Printf("Git root:    %s\n", gitRoot)
				fmt.Printf("Subpath:     %s\n", rel)
			}
		}
	}

	fmt.Printf("Runtime:     %s\n", rt)
	fmt.Printf("Worker:      %s\n", workerName)
	branch := task.Branch
	if branch == "" {
		branch = "(auto-generated)"
	}
	fmt.Printf("Branch:      %s\n", branch)
	fmt.Printf("Session:     %s\n", task.SessionName())
}

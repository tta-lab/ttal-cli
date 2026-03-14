package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/usage"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

var routeToAgent string
var routeMessage string

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
		usage.Log("task.route", routeToAgent)
		rt := cfg.AgentRuntimeFor(routeToAgent)
		role := agent.Role
		if role == "" {
			role = "default"
		}
		prompt := cfg.RenderPrompt(role, uuid, rt)
		if prompt == "" {
			return fmt.Errorf("no prompt for role %q, no [default] in roles.toml, and no fallback in config.toml", role)
		}
		return routeTaskToAgent(routeToAgent, uuid, "task "+role, prompt, routeMessage)
	},
}

func init() {
	taskRouteCmd.Flags().StringVar(&routeToAgent, "to", "", "Agent name to route to (required)")
	_ = taskRouteCmd.MarkFlagRequired("to")
	taskRouteCmd.Flags().StringVar(&routeMessage, "message", "", "Optional context appended to the routing prompt")
}

// buildRoutingRecord constructs the routing annotation for the task audit trail.
// Format: "routed: <from> → <to> [message: <text>]" (message section optional).
// `to` is guaranteed non-empty at all call sites: it is either the required --to flag
// or a role-resolved agent name, both validated before reaching this function.
func buildRoutingRecord(from, to, message string) string {
	sender := from
	if sender == "" {
		sender = "unknown"
	}
	if message != "" {
		return fmt.Sprintf("routed: %s → %s [message: %s]", sender, to, message)
	}
	return fmt.Sprintf("routed: %s → %s", sender, to)
}

// routeTaskToAgent sends a task assignment message to a named agent via the daemon.
func routeTaskToAgent(agentName, taskUUID, roleTag, rolePrompt, message string) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	if task.Status == taskStatusCompleted {
		return fmt.Errorf("task %s is already completed — cannot route", taskUUID)
	}

	uuid := task.UUID
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}

	msg := fmt.Sprintf("[%s] %s — %s\n%s",
		roleTag, uuid, task.Description, rolePrompt)
	if message != "" {
		msg += "\n\nAdditional context: " + message
	}

	sender := os.Getenv("TTAL_AGENT_NAME")
	if err := daemon.Send(daemon.SendRequest{
		From:    sender,
		To:      agentName,
		Message: msg,
	}); err != nil {
		return err
	}

	record := buildRoutingRecord(sender, agentName, message)
	if err := taskwarrior.AnnotateTask(task.UUID, record); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to write routing record (check taskwarrior config and task UUID): %v\n", err)
	}
	fmt.Printf("Routed task %s to %s\n", uuid, agentName)
	return nil
}

// spawnWorkerForTask spawns a worker for a task using the standard spawn flow.
// When yes is false, prints project path + re-run hint and returns a non-zero error.
func spawnWorkerForTask(taskUUID string, yes bool) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	if task.Status == taskStatusCompleted {
		return fmt.Errorf("task %s is already completed — cannot execute", taskUUID)
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

	rt := resolveRuntime(task, cfg)

	workerName := strings.TrimPrefix(task.Branch, "worker/")
	if workerName == "" {
		workerName = task.SessionName()
	}

	if !yes {
		printConfirmHint(task)
		return fmt.Errorf("re-run with --yes to confirm")
	}

	if err := startTaskSafe(task.UUID); err != nil {
		return err
	}

	spawnCfg := worker.SpawnConfig{
		Name:     workerName,
		Project:  task.ProjectPath,
		TaskUUID: task.UUID,
		Worktree: true,
		Runtime:  rt,
		Spawner:  detectSpawner(),
	}

	if image := lookupProjectImage(task.Project); image != "" {
		spawnCfg.UseDocker = true
		spawnCfg.Image = image
	}

	usage.Log("task.execute", taskUUID)
	if err := worker.Spawn(spawnCfg); err != nil {
		return err
	}

	return nil
}

// startTaskSafe starts a taskwarrior task, ignoring "already active" errors.
func startTaskSafe(uuid string) error {
	if err := taskwarrior.StartTask(uuid); err != nil {
		if strings.Contains(err.Error(), "already active") {
			fmt.Fprintf(os.Stderr, "Warning: task is already active in taskwarrior\n")
			return nil
		}
		return fmt.Errorf("task start failed before worker spawn: %w", err)
	}
	return nil
}

// lookupProjectImage returns the container image for a task's project name, or "".
// Tries progressively shorter prefixes: "ttal.pr" → "ttal.pr", "ttal".
func lookupProjectImage(taskProject string) string {
	if taskProject == "" {
		return ""
	}
	store := project.NewStore(config.ResolveProjectsPath())
	parts := strings.Split(taskProject, ".")
	for i := len(parts); i >= 1; i-- {
		candidate := strings.Join(parts[:i], ".")
		proj, err := store.Get(candidate)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: image lookup for %q failed: %v\n", candidate, err)
			continue
		}
		if proj != nil && proj.Image != "" {
			return proj.Image
		}
	}
	return ""
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

func resolveRuntime(task *taskwarrior.Task, cfg *config.Config) runtime.Runtime {
	rt := cfg.WorkerRuntime()
	for _, t := range task.Tags {
		switch t {
		case string(runtime.Codex), "cx":
			rt = runtime.Codex
		case "oc", "opencode":
			fmt.Fprintf(os.Stderr, "warning: +%s tag is no longer supported (OpenCode removed), using default runtime\n", t)
		}
	}
	return rt
}

func printConfirmHint(task *taskwarrior.Task) {
	fmt.Fprintf(os.Stderr, "Project: %s\n", task.ProjectPath)
	fmt.Fprintf(os.Stderr, "⚠ Confirm project path matches your plan before proceeding:\n")
	fmt.Fprintf(os.Stderr, "  ttal task execute %s --yes\n", task.SessionID())
}

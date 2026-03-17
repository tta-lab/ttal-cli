package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/daemon"
	projectPkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/usage"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

const defaultTeam = "default"
const defaultRole = "default"

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
			role = defaultRole
		}
		prompt := cfg.RenderPrompt(role, uuid, rt)
		if prompt == "" {
			return fmt.Errorf("no prompt for role %q, no [default] in roles.toml, and no fallback in config.toml", role)
		}

		// Fetch task early — fail before asking for approval on a doomed operation.
		taskInfo, err := taskwarrior.ExportTask(uuid)
		if err != nil {
			return fmt.Errorf("cannot fetch task %s: %w", uuid, err)
		}
		taskDesc := taskInfo.Description

		// Agent sessions require human approval before routing tasks.
		agentLabel := routeToAgent
		if agent.Emoji != "" {
			agentLabel = agent.Emoji + " " + routeToAgent
		}
		if err := requireHumanApproval(
			"task route",
			fmt.Sprintf("Route task to agent\n\n"+
				"📋 Task: %s\n"+
				"🎯 Target: %s\n"+
				"🏷️ Role: %s",
				taskDesc, agentLabel, role),
		); err != nil {
			return err
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
		return fmt.Errorf("task %s is already completed — cannot route\n\n  Verify: task %s export", taskUUID, taskUUID) //nolint:lll
	}

	uuid := task.UUID
	if len(uuid) > 8 {
		uuid = uuid[:8]
	}

	// Build header: role tag, UUID, description, and project path (if resolvable).
	header := fmt.Sprintf("[%s] %s — %s", roleTag, uuid, task.Description)
	if task.Project != "" {
		if projectPath := projectPkg.ResolveProjectPath(task.Project); projectPath != "" {
			header += fmt.Sprintf("\nProject: %s (%s)", task.Project, projectPath)
		}
	}
	msg := header + "\n" + rolePrompt
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
// In agent sessions (TTAL_AGENT_NAME set), requires human approval via Telegram/Matrix
// buttons before proceeding.
func spawnWorkerForTask(taskUUID string) error {
	if err := taskwarrior.ValidateUUID(taskUUID); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(taskUUID)
	if err != nil {
		return err
	}

	if task.Status == taskStatusCompleted {
		return fmt.Errorf("task %s is already completed — cannot execute\n\n  Check task status: task %s export", taskUUID, taskUUID) //nolint:lll
	}

	sessionName := task.SessionName()
	if tmux.SessionExists(sessionName) {
		return fmt.Errorf("session %s already exists — cannot spawn duplicate\n\n  List active workers: ttal worker list", sessionName) //nolint:lll
	}

	projectPath, err := projectPkg.ResolveProjectPathOrError(task.Project)
	if err != nil {
		return fmt.Errorf("task %s: %w", taskUUID, err)
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	rt := cfg.WorkerRuntime()

	workerName := strings.TrimPrefix(task.Branch, "worker/")
	if workerName == "" {
		workerName = task.SessionName()
	}

	// For human CLI: print project path as a preview before spawning.
	if os.Getenv("TTAL_AGENT_NAME") == "" {
		fmt.Fprintf(os.Stderr, "Project: %s\n", projectPath)
	}

	// Agent sessions require human approval before spawning workers.
	if err := requireHumanApproval(
		"task execute",
		fmt.Sprintf("Spawn worker to execute task\n\n"+
			"📋 Task: %s\n"+
			"📂 Project: %s\n"+
			"🔧 Worker: %s\n"+
			"🌿 Branch: worker/%s",
			task.Description, projectPath, workerName, workerName),
	); err != nil {
		return err
	}

	if err := startTaskSafe(task.UUID); err != nil {
		return err
	}

	spawnCfg := worker.SpawnConfig{
		Name:     workerName,
		Project:  projectPath,
		TaskUUID: task.UUID,
		Worktree: true,
		Runtime:  rt,
		Spawner:  detectSpawner(),
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
		team = defaultTeam
	}
	return team + ":" + agent
}

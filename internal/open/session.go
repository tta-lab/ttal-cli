package open

import (
	"fmt"
	"os"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// Session attaches to the tmux session associated with a task.
// Checks worker session first, then falls back to agent session.
func Session(uuid string) error {
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}

	task, err := taskwarrior.ExportTask(uuid)
	if err != nil {
		return err
	}

	// Try worker session first.
	sessionName := task.SessionName()
	if tmux.SessionExists(sessionName) {
		return attachToSession(sessionName)
	}

	// Fall back to agent session if task has an agent tag.
	// config.Load errors are swallowed intentionally: if config is unavailable
	// (e.g. first-time setup), we degrade gracefully to the "no session" error
	// rather than surfacing an unrelated config problem.
	cfg, err := config.Load()
	if err == nil {
		if agentSession, found := resolveAgentSession(task, cfg.TeamName(), cfg.TeamPath()); found {
			if tmux.SessionExists(agentSession) {
				return attachToSession(agentSession)
			}
		}
	}

	return fmt.Errorf("no worker or agent session for this task\n\n"+
		"  To spawn a worker:\n"+
		"  ttal task go %s", uuid)
}

// resolveAgentSession checks if any of the task's tags match a known agent name.
// Agent tags are lowercase and exactly match the agent filename stem (e.g. +astra → astra.md).
// Returns the agent's session name and true if found, or ("", false) otherwise.
func resolveAgentSession(task *taskwarrior.Task, teamName, teamPath string) (string, bool) {
	if teamPath == "" {
		return "", false
	}
	for _, tag := range task.Tags {
		if agentfs.HasAgent(teamPath, tag) {
			return config.AgentSessionName(teamName, tag), true
		}
	}
	return "", false
}

func attachToSession(sessionName string) error {
	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionName,
	}, os.Environ())
}

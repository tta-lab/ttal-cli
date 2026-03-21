package open

import (
	"errors"
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
	// Swallow os.ErrNotExist (first-time setup, config not yet created).
	// Other errors (corrupted TOML, bad permissions) are surfaced so the user
	// can diagnose the real problem instead of seeing "no session".
	cfg, err := config.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not load config for agent session lookup: %w", err)
	}
	if err == nil {
		if agentSession, found := ResolveAgentSession(task.Tags, cfg.TeamName(), cfg.TeamPath()); found {
			if tmux.SessionExists(agentSession) {
				return attachToSession(agentSession)
			}
		}
	}

	return fmt.Errorf("no worker or agent session for this task\n\n"+
		"  To spawn a worker:\n"+
		"  ttal go %s", uuid)
}

// ResolveAgentSession checks if any of the given tags match a known agent name.
// Agent tags are lowercase and exactly match the agent filename stem (e.g. +astra → astra.md).
// Returns the agent's session name and true if found, or ("", false) otherwise.
func ResolveAgentSession(tags []string, teamName, teamPath string) (string, bool) {
	if teamPath == "" {
		return "", false
	}
	for _, tag := range tags {
		if agentfs.HasAgent(teamPath, tag) {
			return config.AgentSessionName(teamName, tag), true
		}
	}
	return "", false
}

func attachToSession(sessionName string) error {
	tmuxBin, err := lookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}

	return syscall.Exec(tmuxBin, []string{
		"tmux", "attach-session", "-t", sessionName,
	}, os.Environ())
}

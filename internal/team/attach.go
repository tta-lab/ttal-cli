package team

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// Attach attaches the current terminal to an agent's tmux session.
// Accepts "agent" (uses active team) or "team:agent" (explicit team).
// Uses exec (replaces current process) so the user's terminal becomes the session.
func Attach(input string) error {
	var team, agent string
	if parts := strings.SplitN(input, ":", 2); len(parts) == 2 {
		team = parts[0]
		agent = parts[1]
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		team = cfg.TeamName()
		agent = input
	}

	sessionName := config.AgentSessionName(team, agent)

	if !tmux.SessionExists(sessionName) {
		return fmt.Errorf("session %q not found — start with: ttal team start", sessionName)
	}

	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	args := []string{"tmux", "attach-session", "-t", sessionName}
	return syscall.Exec(tmuxBin, args, os.Environ())
}

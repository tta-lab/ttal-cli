package team

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/tmux"
)

// Attach attaches the current terminal to an agent's tmux session.
// Accepts "agent" (uses active team) or "team:agent" (explicit team).
// Uses exec (replaces current process) so the user's terminal becomes the session.
func Attach(input string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	var teamName, agent string
	if parts := strings.SplitN(input, ":", 2); len(parts) == 2 {
		teamName = parts[0]
		agent = parts[1]
	} else {
		teamName = cfg.TeamName()
		agent = input
	}

	if _, ok := cfg.Teams[teamName]; !ok {
		return fmt.Errorf("unknown team: %s", teamName)
	}

	sessionName := config.AgentSessionName(teamName, agent)
	if !tmux.SessionExists(sessionName) {
		return fmt.Errorf("session %q not found — start the daemon with: ttal daemon start", sessionName)
	}

	tmuxBin, err := exec.LookPath("tmux")
	if err != nil {
		return fmt.Errorf("tmux not found in PATH")
	}

	args := []string{"tmux", "attach-session", "-t", sessionName}
	return syscall.Exec(tmuxBin, args, os.Environ())
}

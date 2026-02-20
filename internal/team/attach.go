package team

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/tmux"
)

// Attach attaches the current terminal to an agent's tmux session.
// Uses exec (replaces current process) so the user's terminal becomes the session.
func Attach(agentName string) error {
	sessionName := config.AgentSessionName(agentName)

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

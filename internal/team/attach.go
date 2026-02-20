package team

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"codeberg.org/clawteam/ttal-cli/internal/config"
	"codeberg.org/clawteam/ttal-cli/internal/zellij"
)

// Attach attaches the current terminal to an agent's zellij session.
// Uses exec (replaces current process) so the user's terminal becomes the session.
func Attach(agentName string) error {
	sessionName := config.AgentSessionName(agentName)

	if !zellij.SessionExists(sessionName) {
		return fmt.Errorf("session %q not found — start with: ttal team start", sessionName)
	}

	zellijBin, err := exec.LookPath("zellij")
	if err != nil {
		return fmt.Errorf("zellij not found in PATH")
	}

	args := []string{"zellij", "--data-dir", zellij.DataDir(), "attach", sessionName}
	return syscall.Exec(zellijBin, args, os.Environ())
}

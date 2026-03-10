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
// For k8s teams, uses kubectl exec into the pod's tmux session instead.
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

	team, ok := cfg.Teams[teamName]
	if !ok {
		return fmt.Errorf("unknown team: %s", teamName)
	}

	if team.IsK8s() {
		return attachK8s(team, teamName, agent)
	}

	sessionName := config.AgentSessionName(teamName, agent)
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

// attachK8s execs into a k8s pod's tmux session.
// Inside k8s pods, tmux sessions are named just "<agent>" (not "<team>_<agent>")
// because k8sTeamPod.SpawnAgent uses the bare agent name.
func attachK8s(team config.TeamConfig, teamName, agentName string) error {
	kubectlBin, err := exec.LookPath("kubectl")
	if err != nil {
		return fmt.Errorf("kubectl not found in PATH")
	}

	podName := fmt.Sprintf("ttal-%s", teamName)
	// Namespace is hardcoded to "ttal" — matches daemon k8s.go:325
	args := []string{
		"kubectl",
		"--context", team.Kubernetes.Context,
		"-n", "ttal",
		"exec", "-it", podName,
		"--", "tmux", "attach-session", "-t", agentName,
	}
	return syscall.Exec(kubectlBin, args, os.Environ())
}

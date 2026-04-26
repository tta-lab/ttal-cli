package launchcmd

import (
	"fmt"
	"strings"
)

// BuildCCDirectCommand builds a gatekeeper-wrapped direct claude command using --agent.
func BuildCCDirectCommand(ttalBin, agent, trigger string) string {
	cmd := fmt.Sprintf(
		"%s worker gatekeeper -- claude --dangerously-skip-permissions --agent %s",
		ttalBin, agent,
	)
	if trigger != "" {
		escaped := strings.ReplaceAll(trigger, "'", "'\\''")
		cmd += fmt.Sprintf(" -- '%s'", escaped)
	}
	return cmd
}

// BuildLenosCommand builds a lenos launch command via the worker gatekeeper.
func BuildLenosCommand(ttalBin, agent, trigger string) string {
	cmd := fmt.Sprintf("%s worker gatekeeper -- lenos --agent %s", ttalBin, agent)
	if trigger != "" {
		escaped := strings.ReplaceAll(trigger, "'", "'\\''")
		cmd += fmt.Sprintf(" -- '%s'", escaped)
	}
	return cmd
}

// BuildCodexGatekeeperCommand builds a gatekeeper-wrapped codex command
// using the task-file pattern.
func BuildCodexGatekeeperCommand(ttalBin, taskFile string) (string, error) {
	return fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- codex --yolo --",
		ttalBin, taskFile), nil
}

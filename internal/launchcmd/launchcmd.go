package launchcmd

import (
	"fmt"
	"strings"
)

// BuildCCDirectCommand builds a gatekeeper-wrapped direct claude command using --agent.
// This replaces BuildCCSessionCommand for CC workers and reviewers — context is injected
// via the CC SessionStart hook (ttal context) rather than a synthetic JSONL session.
// agent: CC agent identity (e.g. "coder", "pr-review-lead"). Required.
// trigger: positional arg (the initial message; empty = omit).
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

// BuildCodexGatekeeperCommand builds a gatekeeper-wrapped codex command
// using the legacy task-file pattern. Claude Code uses BuildCCDirectCommand instead.
// This will be removed when Codex supports the context hook (#321).
func BuildCodexGatekeeperCommand(ttalBin, taskFile string) (string, error) {
	return fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- codex --yolo --",
		ttalBin, taskFile), nil
}

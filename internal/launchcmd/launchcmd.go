package launchcmd

import (
	"fmt"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// ContextTrigger is the wake-orientation trigger sent to every spawned or breathed
// CC/Lenos/Codex agent as their first user message. The agent runs `ttal context`
// to render diary, agents, projects, pairing, role prompt, and task in one bundle.
//
// This is the single source of truth for the trigger string — all spawn and breathe
// paths must reference this constant rather than duplicating the literal.
const ContextTrigger = "Run `ttal context` for your briefing, then act on the role prompt."

// BuildCCDirectCommand builds a gatekeeper-wrapped direct claude command using --agent.
// resumeSessionID, if non-empty, appends --resume <id> before the trigger separator.
func BuildCCDirectCommand(ttalBin, agent, trigger, resumeSessionID string) string {
	cmd := fmt.Sprintf(
		"%s worker gatekeeper -- claude --dangerously-skip-permissions --agent %s",
		ttalBin, agent,
	)
	if resumeSessionID != "" {
		cmd += " --resume " + resumeSessionID
	}
	if trigger != "" {
		cmd += " -- " + singleQuoteShell(trigger)
	}
	return cmd
}

// BuildLenosCommand builds a lenos launch command via the worker gatekeeper.
// When readOnly is true, appends --readonly to enforce read-only filesystem
// access via the temenos sandbox.
// resumeSessionID, if non-empty, appends --session <id> before the trigger separator.
func BuildLenosCommand(ttalBin, agent, trigger string, readOnly bool, resumeSessionID string) string {
	cmd := fmt.Sprintf("%s worker gatekeeper -- lenos --agent %s", ttalBin, agent)
	if readOnly {
		cmd += " --readonly"
	}
	if resumeSessionID != "" {
		cmd += " --session " + resumeSessionID
	}
	if trigger != "" {
		cmd += " -- " + singleQuoteShell(trigger)
	}
	return cmd
}

// singleQuoteShell returns s wrapped in single quotes with embedded apostrophes
// escaped as close-quote, escaped-quote, reopen-quote. Safe for use as one shell
// argument inside a command string passed to exec via tmux/sh -c.
// Backticks, $vars, ;, && are all literal inside single quotes.
func singleQuoteShell(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

// BuildEnvParts returns the SSOT env vars every spawned agent receives:
// TTAL_AGENT_NAME, TTAL_JOB_ID, TTAL_RUNTIME.
func BuildEnvParts(taskHexID, agentName string, rt runtime.Runtime) []string {
	return []string{
		"TTAL_AGENT_NAME=" + agentName,
		"TTAL_JOB_ID=" + taskHexID,
		"TTAL_RUNTIME=" + string(rt),
	}
}

// BuildAgentLaunchCommand builds the gatekeeper-wrapped shell command for
// launching an agent at the given runtime. Returns an error for runtimes not
// supported in the worker plane (Codex). Trigger is hardcoded to ContextTrigger —
// every spawned worker-plane agent runs `ttal context` as its wake-orientation.
//
// readOnly is forwarded to lenos via --readonly when rt is Lenos; ignored for
// other runtimes (Claude Code has no equivalent sandbox-enforced flag).
// resumeSessionID, if non-empty, is forwarded as --resume (CC) or --session (Lenos).
func BuildAgentLaunchCommand(
	rt runtime.Runtime, ttalBin, agentName string,
	readOnly bool, trigger, resumeSessionID string,
) (string, error) {
	switch rt {
	case runtime.Lenos:
		return BuildLenosCommand(ttalBin, agentName, trigger, readOnly, resumeSessionID), nil
	case runtime.ClaudeCode:
		return BuildCCDirectCommand(ttalBin, agentName, trigger, resumeSessionID), nil
	default:
		return "", fmt.Errorf("runtime %q is not supported in the worker plane", rt)
	}
}

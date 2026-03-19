package launchcmd

import (
	"fmt"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// BuildResumeCommand builds a gatekeeper-wrapped claude --resume command.
// sessionID: the synthetic JSONL session to resume.
// trigger: one-line prompt passed as positional arg (empty = omit entirely).
// agent: CC agent identity (empty = omit --agent flag).
// Currently only supports ClaudeCode. Codex support tracked in #321.
func BuildResumeCommand(ttalBin, sessionID string, rt runtime.Runtime, model, agent, trigger string) (string, error) {
	if model == "" {
		model = "sonnet"
	}
	switch rt {
	case runtime.ClaudeCode:
		cmd := fmt.Sprintf(
			"%s worker gatekeeper -- claude --resume %s --model %s --dangerously-skip-permissions",
			ttalBin, sessionID, model,
		)
		if agent != "" {
			cmd += fmt.Sprintf(" --agent %s", agent)
		}
		if trigger != "" {
			escaped := strings.ReplaceAll(trigger, "'", "'\\''")
			cmd += fmt.Sprintf(" -- '%s'", escaped)
		}
		return cmd, nil
	default:
		return "", fmt.Errorf("unsupported runtime for resume command: %q (codex support: #321)", rt)
	}
}

// BuildCodexGatekeeperCommand builds a gatekeeper-wrapped codex command
// using the legacy task-file pattern. Claude Code uses BuildResumeCommand instead.
// This will be removed when Codex supports JSONL resume (#321).
func BuildCodexGatekeeperCommand(ttalBin, taskFile string) (string, error) {
	return fmt.Sprintf(
		"%s worker gatekeeper --task-file %s -- codex --yolo --",
		ttalBin, taskFile), nil
}

func BuildGatekeeperCommand(ttalBin, taskFile string, rt runtime.Runtime, model, agent string) (string, error) {
	if model == "" {
		model = "sonnet"
	}
	switch rt {
	case runtime.ClaudeCode:
		cmd := fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- claude --model %s --dangerously-skip-permissions",
			ttalBin, taskFile, model)
		if agent != "" {
			cmd += fmt.Sprintf(" --agent %s", agent)
		}
		cmd += " --"
		return cmd, nil
	case runtime.Codex:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- codex --yolo --",
			ttalBin, taskFile), nil
	}
	return "", fmt.Errorf("unsupported worker runtime for gatekeeper command: %q", rt)
}

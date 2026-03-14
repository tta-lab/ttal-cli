package launchcmd

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

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

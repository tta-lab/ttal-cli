package launchcmd

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

func BuildGatekeeperCommand(ttalBin, taskFile string, rt runtime.Runtime, model string) (string, error) {
	if model == "" {
		model = "sonnet"
	}
	switch rt {
	case runtime.ClaudeCode:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- claude --model %s --dangerously-skip-permissions --",
			ttalBin, taskFile, model), nil
	case runtime.OpenCode:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- opencode --prompt",
			ttalBin, taskFile), nil
	case runtime.Codex:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- codex --yolo --prompt",
			ttalBin, taskFile), nil
	default:
		return "", fmt.Errorf("unsupported worker runtime for gatekeeper command: %q", rt)
	}
}

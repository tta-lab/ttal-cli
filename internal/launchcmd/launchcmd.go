package launchcmd

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Options configures runtime-specific launch flags.
type Options struct {
	ClaudeModel string
	ClaudeYolo  bool
	CodexYolo   bool
}

// BuildGatekeeperCommand builds the runtime command wrapped by `ttal worker gatekeeper`.
// Non-worker runtimes fall back to Claude command shape for safe default behavior.
func BuildGatekeeperCommand(ttalBin, taskFile string, rt runtime.Runtime, opts Options) string {
	switch rt {
	case runtime.OpenCode:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- opencode --prompt",
			ttalBin, taskFile)
	case runtime.Codex:
		yoloFlag := ""
		if opts.CodexYolo {
			yoloFlag = "--yolo "
		}
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- codex %s--prompt",
			ttalBin, taskFile, yoloFlag)
	default:
		model := opts.ClaudeModel
		if model == "" {
			model = "opus"
		}
		yoloFlag := ""
		if opts.ClaudeYolo {
			yoloFlag = "--dangerously-skip-permissions "
		}
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- claude --model %s %s--",
			ttalBin, taskFile, model, yoloFlag)
	}
}

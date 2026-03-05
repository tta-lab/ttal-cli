package launchcmd

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// Options configures runtime-specific launch options.
type Options struct {
	ClaudeModel string
	ClaudeYolo  bool
	CodexYolo   bool
}

// BuildGatekeeperCommand builds the runtime command wrapped by `ttal worker gatekeeper`.
// Returns an error for unsupported runtimes so callers fail fast instead of silently drifting.
func BuildGatekeeperCommand(ttalBin, taskFile string, rt runtime.Runtime, opts Options) (string, error) {
	switch rt {
	case runtime.ClaudeCode:
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
			ttalBin, taskFile, model, yoloFlag), nil
	case runtime.OpenCode:
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- opencode --prompt",
			ttalBin, taskFile), nil
	case runtime.Codex:
		yoloFlag := ""
		if opts.CodexYolo {
			yoloFlag = "--yolo "
		}
		return fmt.Sprintf(
			"%s worker gatekeeper --task-file %s -- codex %s--prompt",
			ttalBin, taskFile, yoloFlag), nil
	default:
		return "", fmt.Errorf("unsupported worker runtime for gatekeeper command: %q", rt)
	}
}

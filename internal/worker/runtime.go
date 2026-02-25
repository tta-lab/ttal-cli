package worker

import (
	"fmt"
	"os/exec"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

// validateRuntime checks that the runtime's binary is available in PATH.
func validateRuntime(rt runtime.Runtime) error {
	var bin string
	switch rt {
	case runtime.OpenCode:
		bin = "opencode"
	case runtime.Codex:
		bin = "codex"
	default:
		bin = "claude"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s runtime requires %q in PATH", rt, bin)
	}
	return nil
}

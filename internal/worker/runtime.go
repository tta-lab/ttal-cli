package worker

import (
	"fmt"
	"os/exec"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// validateRuntime checks that the runtime is valid for workers and its binary is available.
func validateRuntime(rt runtime.Runtime) error {
	if !rt.IsWorkerRuntime() {
		return fmt.Errorf("runtime %q cannot be used for workers (use claude-code or codex)", rt)
	}

	var bin string
	switch rt {
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

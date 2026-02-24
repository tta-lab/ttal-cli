package worker

import (
	"fmt"
	"os/exec"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
)

// Re-export shared runtime types so existing worker callers don't break.
type Runtime = runtime.Runtime

const (
	RuntimeClaudeCode = runtime.ClaudeCode
	RuntimeOpenCode   = runtime.OpenCode
)

// ParseRuntime converts a string to a Runtime, defaulting to claude-code.
func ParseRuntime(s string) (Runtime, error) {
	return runtime.Parse(s)
}

// validateRuntime checks that the runtime's binary is available in PATH.
func validateRuntime(rt Runtime) error {
	var bin string
	switch rt {
	case RuntimeOpenCode:
		bin = "opencode"
	default:
		bin = "claude"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s runtime requires %q in PATH", rt, bin)
	}
	return nil
}

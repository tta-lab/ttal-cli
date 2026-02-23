package worker

import (
	"fmt"
	"os/exec"
)

// Runtime identifies which coding agent backend to use.
type Runtime string

const (
	RuntimeClaudeCode Runtime = "claude-code"
	RuntimeOpenCode   Runtime = "opencode"
)

// ParseRuntime converts a string to a Runtime, defaulting to claude-code.
func ParseRuntime(s string) (Runtime, error) {
	switch s {
	case "", "claude-code", "cc":
		return RuntimeClaudeCode, nil
	case "opencode", "oc":
		return RuntimeOpenCode, nil
	default:
		return "", fmt.Errorf("unknown runtime: %q (valid: claude-code, opencode)", s)
	}
}

// validateRuntime checks that the runtime's binary is available in PATH.
func validateRuntime(runtime Runtime) error {
	var bin string
	switch runtime {
	case RuntimeOpenCode:
		bin = "opencode"
	default:
		bin = "claude"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return fmt.Errorf("%s runtime requires %q in PATH", runtime, bin)
	}
	return nil
}

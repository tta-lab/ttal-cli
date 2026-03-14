package runtime

import (
	"fmt"
	"strings"
)

// Runtime identifies which coding agent backend to use.
type Runtime string

const (
	ClaudeCode Runtime = "claude-code"
	Codex      Runtime = "codex"
)

// All returns all valid runtime values.
func All() []Runtime {
	return []Runtime{ClaudeCode, Codex}
}

// Values returns all valid runtime strings (for ent schema enum).
func Values() []string {
	all := All()
	out := make([]string, len(all))
	for i, r := range all {
		out[i] = string(r)
	}
	return out
}

// Parse converts a string to a Runtime, defaulting to ClaudeCode.
// Accepts aliases: "cc" for claude-code, "cx" for codex.
func Parse(s string) (Runtime, error) {
	switch s {
	case "", string(ClaudeCode), "cc":
		return ClaudeCode, nil
	case string(Codex), "cx":
		return Codex, nil
	default:
		return "", fmt.Errorf("unknown runtime: %q (valid: %s)", s, joinRuntimes())
	}
}

// Validate checks that a string is a valid runtime value.
func Validate(s string) error {
	if !Runtime(s).IsWorkerRuntime() {
		return fmt.Errorf("unknown runtime %q (available: %s)", s, joinRuntimes())
	}
	return nil
}

// IsWorkerRuntime returns true if the runtime can be used for workers.
func (r Runtime) IsWorkerRuntime() bool {
	switch r {
	case ClaudeCode, Codex:
		return true
	default:
		return false
	}
}

func joinRuntimes() string {
	all := All()
	strs := make([]string, len(all))
	for i, r := range all {
		strs[i] = string(r)
	}
	return strings.Join(strs, ", ")
}

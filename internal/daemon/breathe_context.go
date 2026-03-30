package daemon

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"time"
)

const breatheContextTimeout = 30 * time.Second

// evaluateBreatheContext runs breathe_context commands and returns concatenated output.
// Template vars: {{agent-name}} -> agentName, {{team-name}} -> teamName.
// Individual command failures are non-fatal — logged and skipped.
// Returns empty string if no commands produce output.
func evaluateBreatheContext(commands []string, agentName, teamName string) string {
	if len(commands) == 0 {
		return ""
	}

	var b strings.Builder
	for _, cmd := range commands {
		expanded := expandBreatheVars(cmd, agentName, teamName)
		output := runBreatheCommand(expanded)
		if output == "" {
			continue
		}
		b.WriteString("--- ")
		b.WriteString(expanded)
		b.WriteString(" ---\n")
		b.WriteString(output)
		b.WriteString("\n\n")
	}
	return b.String()
}

// expandBreatheVars replaces {{agent-name}} and {{team-name}} in a command string.
func expandBreatheVars(cmd, agentName, teamName string) string {
	cmd = strings.ReplaceAll(cmd, "{{agent-name}}", agentName)
	cmd = strings.ReplaceAll(cmd, "{{team-name}}", teamName)
	return cmd
}

// runBreatheCommand executes a shell command with a timeout and returns its stdout.
// On failure, logs the error and returns empty string.
func runBreatheCommand(cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), breatheContextTimeout)
	defer cancel()

	//nolint:gosec // command comes from user config, not untrusted input
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).Output()
	if err != nil {
		log.Printf("[breathe] context command failed (skipped): %s: %v", cmd, err)
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

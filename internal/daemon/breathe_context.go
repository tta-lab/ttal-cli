package daemon

import (
	"context"
	"log"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const breatheContextTimeout = 30 * time.Second

var templateVarRe = regexp.MustCompile(`\{\{[^}]+\}\}`)

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
		warnUnexpandedVars(expanded)
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
	result := b.String()
	if result == "" {
		log.Printf("[breathe] %s/%s: all breathe_context commands failed — no context produced", agentName, teamName)
	}
	return result
}

// expandBreatheVars replaces {{agent-name}} and {{team-name}} in a command string.
func expandBreatheVars(cmd, agentName, teamName string) string {
	cmd = strings.ReplaceAll(cmd, "{{agent-name}}", agentName)
	cmd = strings.ReplaceAll(cmd, "{{team-name}}", teamName)
	return cmd
}

// warnUnexpandedVars logs a warning for any remaining {{...}} template vars after expansion.
func warnUnexpandedVars(cmd string) {
	if m := templateVarRe.FindString(cmd); m != "" {
		log.Printf("[breathe] command contains unexpanded template var %q — check breathe_context config: %s", m, cmd)
	}
}

// runBreatheCommand executes a shell command with a timeout and returns its stdout+stderr.
// Logs a distinct message for timeouts vs other failures. Returns empty string on failure.
func runBreatheCommand(cmd string) string {
	ctx, cancel := context.WithTimeout(context.Background(), breatheContextTimeout)
	defer cancel()

	//nolint:gosec // command comes from user config, not untrusted input
	out, err := exec.CommandContext(ctx, "sh", "-c", cmd).CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			log.Printf("[breathe] context command timed out (skipped): %s", cmd)
		} else {
			log.Printf("[breathe] context command failed (skipped): %s: %v\n%s", cmd, err, strings.TrimRight(string(out), "\n"))
		}
		return ""
	}
	return strings.TrimRight(string(out), "\n")
}

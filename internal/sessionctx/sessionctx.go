package sessionctx

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

// EvaluateBreatheContext runs breathe_context commands and returns concatenated output.
// Template vars: {{agent-name}} -> agentName, {{team-name}} -> teamName.
// Individual command failures are non-fatal — logged and skipped.
// Returns empty string if no commands produce output.
func EvaluateBreatheContext(commands []string, agentName, teamName string) string {
	if len(commands) == 0 {
		return ""
	}

	var b strings.Builder
	failCount := 0
	for _, cmd := range commands {
		expanded := ExpandBreatheVars(cmd, agentName, teamName)
		WarnUnexpandedVars(expanded)
		output, ok := runBreatheCommandWithStatus(expanded)
		if !ok {
			failCount++
			continue
		}
		if output == "" {
			continue // command succeeded but produced no output (e.g. empty list)
		}
		b.WriteString("--- ")
		b.WriteString(expanded)
		b.WriteString(" ---\n")
		b.WriteString(output)
		b.WriteString("\n\n")
	}
	result := b.String()
	if result == "" {
		if failCount == len(commands) {
			log.Printf("[breathe] %s/%s: all breathe_context commands failed — no context produced", agentName, teamName)
		} else {
			log.Printf("[breathe] %s/%s: breathe_context commands produced no output (%d failed, %d empty)", agentName, teamName, failCount, len(commands)-failCount)
		}
	}
	return result
}

// runBreatheCommandWithStatus executes a shell command and returns (output, ok).
// ok=false means the command failed (non-zero exit or timeout); ok=true with empty string
// means the command succeeded but produced no output.
func runBreatheCommandWithStatus(cmd string) (string, bool) {
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
		return "", false
	}
	return strings.TrimRight(string(out), "\n"), true
}

// ExpandBreatheVars replaces {{agent-name}} and {{team-name}} in a command string.
func ExpandBreatheVars(cmd, agentName, teamName string) string {
	cmd = strings.ReplaceAll(cmd, "{{agent-name}}", agentName)
	cmd = strings.ReplaceAll(cmd, "{{team-name}}", teamName)
	return cmd
}

// WarnUnexpandedVars logs a warning for any remaining {{...}} template vars after expansion.
func WarnUnexpandedVars(cmd string) {
	if m := templateVarRe.FindString(cmd); m != "" {
		log.Printf("[breathe] command contains unexpanded template var %q — check breathe_context config: %s", m, cmd)
	}
}

// RunBreatheCommand executes a shell command with a timeout and returns its stdout+stderr.
// Logs a distinct message for timeouts vs other failures. Returns empty string on failure.
func RunBreatheCommand(cmd string) string {
	out, _ := runBreatheCommandWithStatus(cmd)
	return out
}

package daemon

import "github.com/tta-lab/ttal-cli/internal/sessionctx"

// evaluateBreatheContext delegates to sessionctx.EvaluateBreatheContext.
func evaluateBreatheContext(commands []string, agentName, teamName string) string {
	return sessionctx.EvaluateBreatheContext(commands, agentName, teamName)
}

// expandBreatheVars delegates to sessionctx.ExpandBreatheVars.
func expandBreatheVars(cmd, agentName, teamName string) string {
	return sessionctx.ExpandBreatheVars(cmd, agentName, teamName)
}

// warnUnexpandedVars delegates to sessionctx.WarnUnexpandedVars.
func warnUnexpandedVars(cmd string) {
	sessionctx.WarnUnexpandedVars(cmd)
}

// runBreatheCommand delegates to sessionctx.RunBreatheCommand.
func runBreatheCommand(cmd string) string {
	return sessionctx.RunBreatheCommand(cmd)
}

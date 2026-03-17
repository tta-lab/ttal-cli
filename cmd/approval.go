package cmd

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/daemon"
)

// requireHumanApproval checks if running in an agent session (TTAL_AGENT_NAME set).
// If so, sends a confirmation message to the human via Telegram/Matrix with
// [Approve] [Reject] buttons and blocks until answered or timed out (5min).
//
// Returns nil if approved (or if running as human CLI).
// Returns error if rejected, timed out, or daemon unavailable — all fail-closed.
//
// On timeout: returns a clear message telling the agent to wait for next instruction.
func requireHumanApproval(action, details string) error {
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		return nil // human caller — no gate needed
	}

	question := fmt.Sprintf("🔒 <b>%s</b> requests approval to <b>%s</b>:\n\n%s", agentName, action, details)
	options := []string{"✅ Approve", "❌ Reject"}

	req := daemon.AskHumanRequest{
		Question:  question,
		Options:   options,
		AgentName: agentName,
		// Session intentionally empty — AgentName routes via the agent's bot token.
	}

	result, err := daemon.AskHuman(req)
	if err != nil {
		return fmt.Errorf("approval request failed (is daemon running? is agent %q in config?): %w", agentName, err)
	}

	if result.Skipped {
		return fmt.Errorf("approval timed out (no response within 5 minutes) —" +
			" do not retry, wait for human's next instruction")
	}

	if result.Answer != "✅ Approve" {
		return fmt.Errorf("action rejected by human — do not retry, wait for human's next instruction")
	}

	return nil
}

package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/route"
	"github.com/tta-lab/ttal-cli/internal/sessionctx"
)

// ccHookResponse is the JSON payload expected by CC SessionStart hooks.
type ccHookResponse struct {
	SystemMessage string `json:"systemMessage,omitempty"`
	Continue      bool   `json:"continue"`
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Output CC SessionStart hook JSON with agent context",
	Long: `ttal context is called by the CC SessionStart hook on every new session.

It reads TTAL_AGENT_NAME to determine which agent is starting. If the env var
is not set (non-agent session), it outputs {} and exits 0 — a no-op for the hook.

For agent sessions it:
  1. Loads config to get breathe_context commands and team name
  2. Evaluates breathe_context commands to build session context
  3. Checks for a pending route file (~/.ttal/routing/<agent>.json) and appends
     role prompt and message if present
  4. Outputs {"systemMessage": "<context>", "continue": true}

Always outputs valid JSON — even on config load failures or corrupt route files.`,
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

// noopHook outputs {} — the CC hook no-op response — and returns nil.
// Used for all graceful-degradation paths to keep them as single-site edits.
func noopHook() {
	fmt.Println("{}")
}

// outputJSON writes v as JSON to stdout and returns any marshal error.
func outputJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal hook response: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func runContext(_ *cobra.Command, _ []string) error {
	agentName := os.Getenv("TTAL_AGENT_NAME")
	if agentName == "" {
		// Non-agent session — no-op hook.
		noopHook()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		// Config load failed — degrade gracefully.
		log.Printf("[context] config load failed: %v — outputting empty hook", err)
		noopHook()
		return nil
	}

	teamName := cfg.TeamName()
	if teamName == "" {
		teamName = config.DefaultTeamName
	}

	// Build context from breathe_context commands.
	var systemMsg string
	if cmds := cfg.BreatheContextCommands(); len(cmds) > 0 {
		systemMsg = sessionctx.EvaluateBreatheContext(cmds, agentName, teamName)
	}

	// Consume route file and append routing context if present.
	routeReq, err := route.Consume(agentName)
	if err != nil {
		// Corrupt or unreadable route file — log and skip, still output valid JSON.
		log.Printf("[context] route file error for %s (skipping): %v", agentName, err)
		routeReq = nil
	}
	if routeReq != nil {
		if routeReq.RolePrompt != "" {
			systemMsg += "\n\n---\n\n## New Task Assignment\n\n" + routeReq.RolePrompt
		}
		if routeReq.Message != "" {
			systemMsg += "\n\n" + routeReq.Message
		}
		log.Printf("[context] routing %s to task %s (routed by %s)", agentName, routeReq.TaskUUID, routeReq.RoutedBy)
	}

	if systemMsg == "" {
		// No context to inject — output no-op.
		noopHook()
		return nil
	}

	if err := outputJSON(ccHookResponse{SystemMessage: systemMsg, Continue: true}); err != nil {
		// Degrade gracefully: marshal failure must not cause a non-zero exit that blocks CC startup.
		log.Printf("[context] failed to marshal hook response (falling back to empty): %v", err)
		noopHook()
	}
	return nil
}

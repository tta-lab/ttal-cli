package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/promptrender"
)

// ccHookResponse is the JSON payload for CC SessionStart hooks.
// hookSpecificOutput.additionalContext is injected into Claude's system context.
type ccHookResponse struct {
	HookSpecificOutput *hookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

type hookSpecificOutput struct {
	HookEventName     string `json:"hookEventName"`
	AdditionalContext string `json:"additionalContext"`
}

func newSessionStartOutput(ctx string) *hookSpecificOutput {
	return &hookSpecificOutput{HookEventName: "SessionStart", AdditionalContext: ctx}
}

// hookInput is the JSON payload CC sends to command hooks via stdin.
type hookInput struct {
	AgentType string `json:"agent_type"`
	CWD       string `json:"cwd"`
}

// readHookInput reads the CC hook input JSON from stdin.
// Returns zero-value hookInput on any error (graceful degradation).
func readHookInput() hookInput {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Printf("[context] failed to read hook input from stdin: %v", err)
		return hookInput{}
	}
	if len(data) == 0 {
		return hookInput{}
	}
	var input hookInput
	if err := json.Unmarshal(data, &input); err != nil {
		log.Printf("[context] failed to parse hook input: %v", err)
		return hookInput{}
	}
	return input
}

var contextCmd = &cobra.Command{
	Use:   "context",
	Short: "Output CC SessionStart hook JSON with agent context",
	Long: `ttal context is called by the CC SessionStart hook on every new session.

It reads agent_type and cwd from the hook's stdin JSON to determine which agent
is starting. If agent_type is absent (non-agent session), it outputs {} and
exits 0 — a no-op for the hook.

For agent sessions it:
  1. Loads config to get the 'context' prompt template
  2. Derives agent identity from agent_type and cwd:
     - Worker (cwd under ~/.ttal/worktrees/): sets TTAL_JOB_ID from worktree dir name
     - Manager: sets TTAL_AGENT_NAME from agent_type only
  3. Renders the context template — $ cmd lines are executed with agent env vars
  4. Outputs {"hookSpecificOutput": {"hookEventName": "SessionStart", "additionalContext": "<context>"}}

Always outputs valid JSON — even on config load failures.`,
	RunE: runContext,
}

func init() {
	rootCmd.AddCommand(contextCmd)
}

// noopHook outputs {} — the CC hook no-op response — and returns nil.
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
	input := readHookInput()

	// agent_type is set by CC when the session uses --agent <name>.
	agentName := input.AgentType
	if agentName == "" {
		// Non-agent session — no-op hook.
		noopHook()
		return nil
	}

	cfg, err := config.Load()
	if err != nil {
		log.Printf("[context] config load failed: %v — outputting empty hook", err)
		noopHook()
		return nil
	}

	teamName := "default"
	if teamName == "" {
		teamName = config.DefaultTeamName
	}

	tmpl := cfg.Prompt("context")
	if tmpl == "" {
		log.Printf("[context] agent=%s: no context template configured", agentName)
		noopHook()
		return nil
	}

	// Derive identity env vars for subprocess execution in the template renderer.
	env := buildContextEnv(agentName, input.CWD)

	output := promptrender.RenderTemplate(tmpl, agentName, teamName, env)
	if output == "" {
		log.Printf("[context] agent=%s: context template produced no output", agentName)
		noopHook()
		return nil
	}

	resp := ccHookResponse{
		HookSpecificOutput: newSessionStartOutput(output),
	}
	if err := outputJSON(resp); err != nil {
		log.Printf("[context] failed to marshal hook response (falling back to empty): %v", err)
		noopHook()
	}
	return nil
}

// buildContextEnv constructs the env slice for subprocess execution in the context template.
// Workers (cwd under ~/.ttal/worktrees/) get TTAL_JOB_ID derived from the worktree dir name.
// All sessions get TTAL_AGENT_NAME from agent_type.
func buildContextEnv(agentName, cwd string) []string {
	env := []string{
		"TTAL_AGENT_NAME=" + agentName,
	}
	if hexID := extractWorktreeHexID(cwd); hexID != "" {
		env = append(env, "TTAL_JOB_ID="+hexID)
	}
	return env
}

// extractWorktreeHexID extracts the hex task ID from a worktree CWD path.
// Worktree dirs follow the pattern ~/.ttal/worktrees/<hexID>-<alias>.
// Returns empty string for non-worktree paths or on error.
func extractWorktreeHexID(cwd string) string {
	if cwd == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("[context] could not resolve home dir (worker treated as manager): %v", err)
		return ""
	}
	worktreesRoot := filepath.Join(home, ".ttal", "worktrees")
	if !strings.HasPrefix(cwd, worktreesRoot+string(filepath.Separator)) {
		return ""
	}
	// cwd is under worktreesRoot — the next path segment is the worktree dir name.
	rel := strings.TrimPrefix(cwd, worktreesRoot+string(filepath.Separator))
	// rel may be "ec16980f-ttal" or "ec16980f-ttal/subdir" — take first segment.
	dirName := strings.SplitN(rel, string(filepath.Separator), 2)[0]
	// dirName format: "<hexID>-<alias>" — hex ID is the part before the first "-".
	parts := strings.SplitN(dirName, "-", 2)
	return parts[0]
}

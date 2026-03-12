package tools

import (
	"context"
	"fmt"

	"charm.land/fantasy"
	"github.com/tta-lab/ttal-cli/pkg/agentloop/sandbox"
)

// BashParams are the input parameters for the bash tool.
type BashParams struct {
	Command     string `json:"command"     description:"The bash command to execute"`
	Description string `json:"description" description:"Brief description of what the command does"`
}

// NewBashTool creates a sandboxed bash tool for use in the agent.
func NewBashTool(sbx sandbox.Sandbox) fantasy.AgentTool {
	return fantasy.NewAgentTool(
		"bash",
		schemaDescription(bashDescription),
		func(ctx context.Context, params BashParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			execCfg := sandbox.ExecConfigFromContext(ctx)

			stdoutStr, stderrStr, exitCode, err := sbx.Exec(ctx, params.Command, execCfg)
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("execution error: %v", err)), nil
			}

			output := stdoutStr
			if stderrStr != "" {
				output += "\nSTDERR:\n" + stderrStr
			}
			if exitCode != 0 {
				output += fmt.Sprintf("\n(exit code: %d)", exitCode)
			}
			if output == "" {
				output = "(no output)"
			}
			return fantasy.NewTextResponse(output), nil
		},
	)
}

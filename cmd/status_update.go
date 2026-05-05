package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/daemon"
)

// postStepEnvelope is the lenos hook's stdin contract (v1).
// Schema is locked by lenos (orientation flicknote 518a51ac); this consumer
// cherry-picks the schema-mapped subset. Unknown / forward-compat fields are
// accepted by json.Unmarshal and silently dropped.
type postStepEnvelope struct {
	Version             int     `json:"version"`
	SessionID           string  `json:"session_id"`
	ModelID             string  `json:"model_id"`
	ContextUsedPct      float64 `json:"context_used_pct"`
	ContextRemainingPct float64 `json:"context_remaining_pct"`
}

var statusUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Read a post-step JSON envelope from stdin and POST it to the daemon",
	Long: `Reads a v1 JSON envelope from stdin and posts a status update to the
daemon's /status/update endpoint. Identity is read from TTAL_AGENT_NAME env
(set at spawn by ttal). Intended to be invoked from lenos's [hooks].post_step.`,
	Hidden:       true,
	Args:         cobra.NoArgs,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		return statusUpdateFromReader(os.Stdin, os.Getenv("TTAL_AGENT_NAME"))
	},
}

func init() {
	statusCmd.AddCommand(statusUpdateCmd)
}

// statusUpdateFromReader is the testable core: reads a v1 envelope from r,
// validates identity + version, maps the subset to StatusUpdateRequest, and
// POSTs via daemon.StatusUpdate.
func statusUpdateFromReader(r io.Reader, agent string) error {
	if agent == "" {
		return fmt.Errorf("ttal status update: missing TTAL_AGENT_NAME env (must run inside a spawned manager-plane session)")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(data) == 0 {
		return fmt.Errorf("ttal status update: empty stdin (expected JSON envelope)")
	}

	var env postStepEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("parse envelope: %w", err)
	}

	if env.Version != 1 {
		return fmt.Errorf("ttal status update: unsupported envelope version %d (want 1)", env.Version)
	}

	return daemon.StatusUpdate(daemon.StatusUpdateRequest{
		Type:                "statusUpdate",
		Agent:               agent,
		ContextUsedPct:      env.ContextUsedPct,
		ContextRemainingPct: env.ContextRemainingPct,
		ModelID:             env.ModelID,
		SessionID:           env.SessionID,
	})
}

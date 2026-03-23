package cmd

import (
	"fmt"
	"os"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
)

// coderWindowName is the fixed window name for coder sessions.
const coderWindowName = "coder"

// resolveReviewerWindow returns the reviewer agent name (= window name) for a
// given pipeline assignee role and task tags. Falls back to fallback.
func resolveReviewerWindow(taskTags []string, assigneeRole, fallback string) string {
	pipelineCfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not load pipelines.toml — falling back to %s: %v\n", fallback, err)
		return fallback
	}
	if name := pipelineCfg.ReviewerForStage(taskTags, assigneeRole); name != "" {
		return name
	}
	return fallback
}

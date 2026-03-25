package cmd

import (
	"fmt"
	"strings"

	"github.com/tta-lab/logos"
)

// flushAgentResult prints a trailing newline if the response didn't end with one,
// then returns the error with an actionable hint when the step limit is reached.
func flushAgentResult(result *logos.RunResult, err error) error {
	if result != nil && result.Response != "" && !strings.HasSuffix(result.Response, "\n") {
		fmt.Println()
	}
	if err != nil {
		if strings.Contains(err.Error(), "max steps") {
			return fmt.Errorf("agent loop: %w\n\nTip: increase the limit with --max-steps", err)
		}
		return fmt.Errorf("agent loop: %w", err)
	}
	return nil
}

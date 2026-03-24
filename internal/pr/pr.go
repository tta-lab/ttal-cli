package pr

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// PRIndex returns the PR index from the task's PRID UDA.
func PRIndex(ctx *Context) (int64, error) {
	if ctx.Task.PRID == "" {
		return 0, fmt.Errorf("no PR associated with this task (create one first with: ttal pr create)")
	}
	info, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return 0, err
	}
	return info.Index, nil
}

package pr

import (
	"errors"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// ErrNoPR is returned when no PR has been associated with the task yet.
// Callers can use errors.Is to distinguish this expected case from
// unexpected infrastructure failures.
var ErrNoPR = errors.New("no PR associated with this task (create one first with: ttal pr create)")

// PRIndex returns the PR index from the task's PRID UDA.
func PRIndex(ctx *Context) (int64, error) {
	if ctx.Task.PRID == "" {
		return 0, ErrNoPR
	}
	info, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		return 0, err
	}
	return info.Index, nil
}

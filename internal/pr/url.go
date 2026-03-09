package pr

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// BuildPRURL constructs the full PR URL from the PR context.
func BuildPRURL(ctx *Context) string {
	if ctx.Task.PRID == "" {
		return ""
	}
	info, err := taskwarrior.ParsePRID(ctx.Task.PRID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: invalid pr_id %q: %v\n", ctx.Task.PRID, err)
		return ""
	}
	return ctx.Info.PRURL(strconv.FormatInt(info.Index, 10))
}

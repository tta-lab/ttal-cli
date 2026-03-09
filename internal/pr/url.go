package pr

import (
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
		return ""
	}
	return ctx.Info.PRURL(strconv.FormatInt(info.Index, 10))
}

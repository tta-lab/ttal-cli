package pr

// BuildPRURL constructs the full PR URL from the PR context.
func BuildPRURL(ctx *Context) string {
	if ctx.Task.PRID == "" {
		return ""
	}
	return ctx.Info.PRURL(ctx.Task.PRID)
}

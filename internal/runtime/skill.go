package runtime

// FormatSkillInvocation returns the skill invocation string for the given runtime.
func FormatSkillInvocation(rt Runtime, skillName string) string {
	if rt == Codex {
		return "$" + skillName
	}
	return "Use " + skillName + " skill"
}

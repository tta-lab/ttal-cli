package runtime

// FormatSkillInvocation returns the skill invocation string for the given runtime.
func FormatSkillInvocation(rt Runtime, skillName string) string {
	switch rt {
	case Codex:
		return "$" + skillName
	default:
		return "Use " + skillName + " skill"
	}
}

package runtime

// FormatSkillInvocation returns the skill invocation string for the given runtime.
// Examples:
//
//	CC/OC: "Use triage skill"
//	Codex: "$triage"
func FormatSkillInvocation(rt Runtime, skillName string) string {
	switch rt {
	case Codex:
		return "$" + skillName
	default:
		// CC and OpenCode: text mention at start
		return "Use " + skillName + " skill"
	}
}

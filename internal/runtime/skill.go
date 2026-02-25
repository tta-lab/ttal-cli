package runtime

import "fmt"

// FormatSkillInvocation returns the skill invocation string for the given runtime.
// Examples:
//
//	CC:    "/triage"
//	OC:    "/triage"
//	Codex: "$triage"
func FormatSkillInvocation(rt Runtime, skillName string) string {
	switch rt {
	case Codex:
		return "$" + skillName
	default:
		return "/" + skillName
	}
}

// FormatSkillMessage prepends the runtime-appropriate skill invocation to the message.
func FormatSkillMessage(rt Runtime, skillName, message string) string {
	skill := FormatSkillInvocation(rt, skillName)
	return fmt.Sprintf("%s %s", skill, message)
}

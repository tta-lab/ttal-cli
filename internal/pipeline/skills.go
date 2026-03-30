package pipeline

import (
	"strings"

	"github.com/tta-lab/ttal-cli/internal/runtime"
)

// CoderAssignee is the pipeline stage assignee value that identifies a worker/coder stage.
// Used by both the daemon advance logic and the pipeline prompt command to identify coder stages.
const CoderAssignee = "coder"

// PrependSkills prepends skill invocations from stage config to a role prompt.
// Returns the prompt unchanged when skills is empty.
func PrependSkills(rolePrompt string, skills []string, rt runtime.Runtime) string {
	if len(skills) == 0 {
		return rolePrompt
	}
	lines := make([]string, len(skills))
	for i, s := range skills {
		lines[i] = runtime.FormatSkillInvocation(rt, s)
	}
	return strings.Join(lines, "\n") + "\n\n" + rolePrompt
}

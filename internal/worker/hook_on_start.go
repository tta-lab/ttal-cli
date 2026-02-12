package worker

import (
	"fmt"
	"os"
	"strings"
)

// HookOnStart handles the task start (+ACTIVE) event.
// Reads two JSON lines from stdin, outputs modified task to stdout.
func HookOnStart() {
	original, modified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-start: " + err.Error())
		os.Exit(0)
	}

	// Only handle transitions to +ACTIVE
	if original.Start() != "" || modified.Start() == "" || modified.Status() != "pending" {
		outputModifiedTask(modified)
		return
	}

	handleOnStart(original, modified)
}

// handleOnStart contains the start logic, callable from HookOnStart or HookOnModify.
func handleOnStart(_ hookTask, modified hookTask) {
	defer outputModifiedTask(modified)

	hookLog("START", modified.UUID(), modified.Description())

	// Research tasks → trigger research agent
	if modified.HasTag("research") {
		hookLog("RESEARCH_DETECT", modified.UUID(), modified.Description())
		triggerResearch(modified)
		return
	}

	// Regular tasks → notify agent for spawn decision
	message := extractTaskContext(modified)
	notifyAgent(message)
	hookLog("AGENT_NOTIFY", modified.UUID(), modified.Description(), "reason", "task_started")
}

func triggerResearch(task hookTask) {
	annotations := task.Annotations()
	var annLines []string
	for _, ann := range annotations {
		if desc, ok := ann["description"].(string); ok {
			annLines = append(annLines, "  - "+desc)
		}
	}

	prompt := fmt.Sprintf(`Research Task

**Task:** %s
**Project:** %s
**Tags:** %s

**Context:**
%s

Please research this topic thoroughly and provide:
1. Key findings and recommendations
2. Technical details and specifications
3. Best practices and pitfalls
4. Implementation suggestions

Format your response as markdown with clear sections.`,
		task.Description(), task.Project(),
		strings.Join(task.Tags(), " "),
		strings.Join(annLines, "\n"))

	notifyAgentWith(prompt, "research-agent")
	hookLog("RESEARCH_TRIGGER", task.UUID(), task.Description())
}

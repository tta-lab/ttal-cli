package worker

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/enrichment"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// enrichInline resolves project_path and branch directly on the hookTask map.
// Sets fields in-place so writeTask outputs the enriched version.
// Errors are logged but never propagated.
func enrichInline(task hookTask) {
	projectPath := project.ResolveProjectPath(task.Project())
	if projectPath == "" {
		hookLogFile(fmt.Sprintf("enrich-inline: no project match for %q (task %s)", task.Project(), task.UUID()))
		return
	}

	task["project_path"] = projectPath

	branch := enrichment.GenerateBranch(task.Description())
	if branch == "" {
		hookLogFile(fmt.Sprintf("enrich-inline: could not generate branch for %s: %q", task.UUID(), task.Description()))
		hookLog("ENRICH", task.UUID(), task.Description(), "project_path", projectPath, "branch", "(none)")
		return
	}

	branchWithPrefix := "worker/" + branch
	task["branch"] = branchWithPrefix

	hookLog("ENRICH", task.UUID(), task.Description(), "project_path", projectPath, "branch", branchWithPrefix)
}

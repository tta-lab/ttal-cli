package worker

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/enrichment"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// enrichInline validates the project alias and generates a branch.
// Sets branch in-place so writeTask outputs the enriched version.
// Returns an error if the project alias is not registered.
func enrichInline(task hookTask) error {
	projectAlias := task.Project()
	if projectAlias == "" {
		return nil // no project set — nothing to validate
	}

	if _, err := project.ResolveProjectPathOrError(projectAlias); err != nil {
		return err
	}

	branch := enrichment.GenerateBranch(task.Description())
	if branch == "" {
		hookLogFile(fmt.Sprintf("enrich-inline: could not generate branch for %s: %q", task.UUID(), task.Description()))
		hookLog("ENRICH", task.UUID(), task.Description(), "branch", "(none)")
		return nil
	}

	branchWithPrefix := "worker/" + branch
	task["branch"] = branchWithPrefix

	hookLog("ENRICH", task.UUID(), task.Description(), "branch", branchWithPrefix)
	return nil
}

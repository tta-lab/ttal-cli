package worker

import (
	"fmt"

	"github.com/tta-lab/ttal-cli/internal/enrichment"
	"github.com/tta-lab/ttal-cli/internal/project"
)

// enrichInline validates the project alias and generates a branch.
// Sets branch in-place so writeTask outputs the enriched version.
// Returns an error if the project alias is not registered.
// resolver may be nil, in which case project.ResolveProjectPath is used.
// Error messages always come from project.ResolveProjectPathOrError so
// production and test paths produce the same user-facing text.
func enrichInline(task hookTask, resolver pathResolver) error {
	projectAlias := task.Project()
	if projectAlias == "" {
		return nil // no project set — nothing to validate
	}

	if resolver == nil {
		resolver = project.ResolveProjectPath
	}

	if resolver(projectAlias) == "" {
		// Delegate to ResolveProjectPathOrError for the user-friendly error message —
		// same text whether called from production (nil resolver) or tests (mock resolver).
		_, err := project.ResolveProjectPathOrError(projectAlias)
		if err != nil {
			return err
		}
		return fmt.Errorf("project %q not found in projects.toml", projectAlias)
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

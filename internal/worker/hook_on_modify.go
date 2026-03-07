package worker

import (
	"fmt"
	"os"
	"strconv"

	"github.com/tta-lab/ttal-cli/internal/gitprovider"
)

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	original, modified, rawModified, err := readHookInput()
	if err != nil {
		hookLogFile("ERROR in on-modify: " + err.Error())
		if len(rawModified) > 0 {
			fmt.Println(string(rawModified))
		}
		os.Exit(0)
	}

	// Check if task is being completed
	if modified.Status() == taskStatusCompleted && original.Status() != taskStatusCompleted {
		if err := validateTaskCompletion(modified, nil); err != nil {
			hookLogFile("ERROR: " + err.Error())
			fmt.Println(err.Error())
			os.Exit(1)
		}
	}

	// Re-enrich when project changes to a non-empty value.
	if newProject := modified.Project(); newProject != "" && newProject != original.Project() {
		enrichInline(modified)
	}

	writeTask(modified)
}

// prMergedChecker is a function that checks whether a PR is merged.
// It receives the project path and PR ID string and returns (merged bool, err error).
// Injected for testability; production code uses defaultPRMergedChecker.
type prMergedChecker func(projectPath, prID string) (merged bool, err error)

// defaultPRMergedChecker is the real implementation used in production.
func defaultPRMergedChecker(projectPath, prID string) (bool, error) {
	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	prIndex, err := strconv.ParseInt(prID, 10, 64)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%s: invalid pr_id", prID)
	}

	pr, err := provider.GetPR(info.Owner, info.Repo, prIndex)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	return pr.Merged, nil
}

// validateTaskCompletion checks if a task can be completed.
// It blocks completion if the task has an unmerged PR.
// checker may be nil, in which case defaultPRMergedChecker is used.
func validateTaskCompletion(modified hookTask, checker prMergedChecker) error {
	prID := modified.PRID()
	if prID == "" {
		return nil
	}

	projectPath := modified.ProjectPath()
	if projectPath == "" {
		return fmt.Errorf("cannot verify PR: task has pr_id but no project_path. " +
			"Add project_path or remove pr_id to complete")
	}

	if checker == nil {
		checker = defaultPRMergedChecker
	}

	merged, err := checker(projectPath, prID)
	if err != nil {
		return err
	}

	if merged {
		return nil
	}

	return fmt.Errorf("cannot complete task with unmerged PR #%s. Merge the PR first", prID)
}

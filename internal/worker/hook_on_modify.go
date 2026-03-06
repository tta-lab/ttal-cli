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
		if err := validateTaskCompletion(modified); err != nil {
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

// validateTaskCompletion checks if a task can be completed.
// It blocks completion if the task has an unmerged PR.
func validateTaskCompletion(modified hookTask) error {
	prID := modified.PRID()
	if prID == "" {
		return nil
	}

	projectPath := modified.ProjectPath()
	if projectPath == "" {
		return fmt.Errorf("cannot verify PR: task has pr_id but no project_path. " +
			"Add project_path or remove pr_id to complete")
	}

	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	prIndex, err := strconv.ParseInt(prID, 10, 64)
	if err != nil {
		return fmt.Errorf("cannot verify PR #%s: invalid pr_id", prID)
	}

	pr, err := provider.GetPR(info.Owner, info.Repo, prIndex)
	if err != nil {
		return fmt.Errorf("cannot verify PR #%s: %w", prID, err)
	}

	if pr.Merged {
		return nil
	}

	return fmt.Errorf("cannot complete task with unmerged PR #%s. Merge the PR first", prID)
}

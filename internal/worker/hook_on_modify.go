package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// taskCompletePayload mirrors daemon.TaskCompleteRequest for serialization.
// Defined here to avoid a worker→daemon circular import.
type taskCompletePayload struct {
	Type     string `json:"type"`
	TaskUUID string `json:"task_uuid"`
	Team     string `json:"team,omitempty"`
	Spawner  string `json:"spawner,omitempty"`
	Desc     string `json:"desc,omitempty"`
	PRID     string `json:"pr_id,omitempty"`
}

// notifyTaskComplete sends a taskComplete HTTP request to the daemon.
// Fire-and-forget: daemon unreachable silently skipped so task completion never blocks.
func notifyTaskComplete(task hookTask) {
	team := os.Getenv("TTAL_TEAM")
	if team == "" {
		team = "default"
	}
	msg := taskCompletePayload{
		Type:     "taskComplete",
		TaskUUID: task.UUID(),
		Team:     team,
		Spawner:  task.Spawner(),
		Desc:     task.Description(),
		PRID:     task.PRID(),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		hookLogFile("taskComplete: marshal failed: " + err.Error())
		return
	}

	sockPath := config.SocketPath()
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sockPath, 3*time.Second)
			},
		},
	}
	resp, err := client.Post("http://daemon/task/complete", "application/json", bytes.NewReader(payload))
	if err != nil {
		hookLogFile("taskComplete: daemon unreachable: " + err.Error())
		return
	}
	resp.Body.Close() //nolint:errcheck // fire-and-forget
}

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
		// Notify daemon — fire-and-forget, won't block task completion
		notifyTaskComplete(modified)
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
	prInfo, err := taskwarrior.ParsePRID(prID)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR %q: %w", prID, err)
	}

	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%d: %w", prInfo.Index, err)
	}

	provider, err := gitprovider.NewProvider(info)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%d: %w", prInfo.Index, err)
	}

	pr, err := provider.GetPR(info.Owner, info.Repo, prInfo.Index)
	if err != nil {
		return false, fmt.Errorf("cannot verify PR #%d: %w", prInfo.Index, err)
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

	prInfo, err := taskwarrior.ParsePRID(prID)
	if err != nil {
		return fmt.Errorf("cannot verify PR %q: %w", prID, err)
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

	return fmt.Errorf("cannot complete task with unmerged PR #%d. Merge the PR first", prInfo.Index)
}

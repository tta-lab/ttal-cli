package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/project"
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
	PRTitle  string `json:"pr_title,omitempty"`
}

// notifyTaskComplete sends a taskComplete HTTP request to the daemon.
// Fire-and-forget: daemon unreachable silently skipped so task completion never blocks.
func notifyTaskComplete(task hookTask, prTitle string) {
	msg := taskCompletePayload{
		Type:     "taskComplete",
		TaskUUID: task.UUID(),
		Team:     config.DefaultTeamName,
		Spawner:  task.Spawner(),
		Desc:     task.Description(),
		PRID:     task.PRID(),
		PRTitle:  prTitle,
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

// matchedPipeline loads pipeline config and returns the pipeline matched by the task's tags.
// Returns nil (no error) when no config exists or no pipeline matches — callers treat nil as "no gate".
// configDir may be empty — defaults to config.DefaultConfigDir().
func matchedPipeline(task hookTask, configDir string) *pipeline.Pipeline {
	if configDir == "" {
		configDir = config.DefaultConfigDir()
	}
	pipelineCfg, err := pipeline.Load(configDir)
	if err != nil || pipelineCfg == nil {
		return nil
	}
	_, p, err := pipelineCfg.MatchPipeline(task.Tags())
	if err != nil {
		return nil
	}
	return p
}

// resolveAllowedReviewers loads the pipeline config from configDir and collects
// all reviewer agent names from stages matching the task's tags.
// Returns nil if no pipeline matches (guard will reject all agents).
func resolveAllowedReviewers(task hookTask, configDir string) []string {
	p := matchedPipeline(task, configDir)
	if p == nil {
		return nil
	}
	var reviewers []string
	for _, stage := range p.Stages {
		if stage.Reviewer != "" {
			reviewers = append(reviewers, stage.Reviewer)
		}
	}
	return reviewers
}

// lgtmTagAdded returns the _lgtm tag that was added (empty if none).
func lgtmTagAdded(originalTags, modifiedTags []string) string {
	origSet := make(map[string]bool, len(originalTags))
	for _, t := range originalTags {
		origSet[t] = true
	}
	for _, t := range modifiedTags {
		if taskwarrior.IsLGTMTag(t) && !origSet[t] {
			return t
		}
	}
	return ""
}

// checkPipelineDoneGuard blocks task completion when the task matches a pipeline
// but the last stage hasn't been approved. Pipeline completion is determined by
// the presence of +<laststage>_lgtm tag.
// configDir may be empty — defaults to config.DefaultConfigDir().
func checkPipelineDoneGuard(task hookTask, configDir string) error {
	p := matchedPipeline(task, configDir)
	if p == nil {
		return nil
	}
	lastStage := p.LastStage()
	if lastStage == nil {
		return nil
	}
	if slices.Contains(task.Tags(), lastStage.StageLGTMTag()) {
		return nil
	}
	// Allow completion if last stage has no reviewer (e.g. research, audit pipelines).
	if lastStage.Reviewer == "" {
		if slices.Contains(task.Tags(), lastStage.StageTag()) {
			return nil
		}
	}
	return fmt.Errorf(
		"cannot complete task: pipeline not finished (need +%s). Use `ttal go <uuid>` to advance through stages",
		lastStage.StageLGTMTag(),
	)
}

// checkLGTMGuard rejects _lgtm tag additions from agents not listed as pipeline reviewers.
// addedLgtmTag must be pre-computed by the caller to avoid duplicating the predicate.
// allowedReviewers is collected from the pipeline stages' Reviewer fields.
// If allowedReviewers is nil (no pipeline found), all agents are rejected.
func checkLGTMGuard(addedLgtmTag string, allowedReviewers []string) error {
	if addedLgtmTag == "" {
		return nil
	}
	agent := os.Getenv("TTAL_AGENT_NAME")
	if slices.Contains(allowedReviewers, agent) {
		return nil
	}
	return fmt.Errorf(
		"only pipeline reviewers can set +%s (current agent: %s, allowed: %v)",
		addedLgtmTag, agent, allowedReviewers,
	)
}

// HookOnModify is the main taskwarrior on-modify hook entry point.
func HookOnModify() {
	original, modified, rawModified, err := readHookInput()
	if err != nil {
		// Parse-error fallback: echo modified JSON back so taskwarrior can still write the task.
		// This is intentional — do not convert to exit 1 here; malformed input should not block writes.
		hookLogFile("ERROR in on-modify: " + err.Error())
		if len(rawModified) > 0 {
			fmt.Println(string(rawModified))
		}
		os.Exit(0)
	}

	// Guard: only pipeline reviewers can set _lgtm tags.
	// Compute addedLgtmTag once — used to gate the pipeline load and passed to the guard.
	addedLgtmTag := lgtmTagAdded(original.Tags(), modified.Tags())
	var allowedReviewers []string
	if addedLgtmTag != "" {
		allowedReviewers = resolveAllowedReviewers(modified, config.DefaultConfigDir())
	}
	if err := checkLGTMGuard(addedLgtmTag, allowedReviewers); err != nil {
		hookLogFile("LGTM guard: " + err.Error())
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	// Check if task is being completed
	if modified.Status() == taskStatusCompleted && original.Status() != taskStatusCompleted {
		// Block completion if pipeline stages are incomplete.
		if err := checkPipelineDoneGuard(modified, ""); err != nil {
			hookLogFile("pipeline guard: " + err.Error())
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		prTitle, err := validateTaskCompletion(modified, nil, nil)
		if err != nil {
			hookLogFile("ERROR: " + err.Error())
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
		// Notify daemon — fire-and-forget, won't block task completion
		notifyTaskComplete(modified, prTitle)
	}

	// Re-enrich when project changes to a non-empty value.
	if newProject := modified.Project(); newProject != "" && newProject != original.Project() {
		if err := enrichInline(modified, nil); err != nil {
			hookLogFile("ERROR: " + err.Error())
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	writeTask(modified)
}

// prMergedChecker checks whether a PR is merged and returns its title.
// It receives the project alias, project path, and PR ID string.
// Injected for testability; production code uses defaultPRMergedChecker.
type prMergedChecker func(projectAlias, projectPath, prID string) (merged bool, title string, err error)

// pathResolver resolves a project alias to a filesystem path.
// Injected for testability; production code uses project.ResolveProjectPath.
type pathResolver func(projectName string) string

// prGetPRPayload mirrors daemon.PRGetPRRequest for socket serialization.
// Defined here to avoid a worker→daemon circular import.
type prGetPRPayload struct {
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	Index        int64  `json:"index"`
	ProjectAlias string `json:"project_alias,omitempty"`
}

// prGetPRResult mirrors daemon.PRGetPRResponse for socket deserialization.
// Defined here to avoid a worker→daemon circular import.
type prGetPRResult struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Merged bool   `json:"merged,omitempty"`
	Title  string `json:"title,omitempty"`
}

// defaultPRMergedChecker is the real implementation used in production.
// Proxies through the daemon's POST /pr/get endpoint so per-project tokens are resolved centrally.
func defaultPRMergedChecker(projectAlias, projectPath, prID string) (bool, string, error) {
	prInfo, err := taskwarrior.ParsePRID(prID)
	if err != nil {
		return false, "", fmt.Errorf("cannot verify PR %q: %w", prID, err)
	}

	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return false, "", fmt.Errorf("cannot verify PR #%d: %w", prInfo.Index, err)
	}

	payload := prGetPRPayload{
		ProviderType: string(info.Provider),
		Owner:        info.Owner,
		Repo:         info.Repo,
		Index:        prInfo.Index,
		ProjectAlias: projectAlias,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return false, "", fmt.Errorf("cannot verify PR #%d: marshal: %w", prInfo.Index, err)
	}

	sockPath := config.SocketPath()
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", sockPath, 5*time.Second)
			},
		},
	}
	httpResp, err := client.Post("http://daemon/pr/get", "application/json", bytes.NewReader(body))
	if err != nil {
		return false, "", fmt.Errorf("cannot verify PR #%d: daemon unreachable: %w", prInfo.Index, err)
	}
	defer httpResp.Body.Close() //nolint:errcheck

	var result prGetPRResult
	if err := json.NewDecoder(httpResp.Body).Decode(&result); err != nil {
		return false, "", fmt.Errorf("cannot verify PR #%d: decode response: %w", prInfo.Index, err)
	}
	if !result.OK {
		return false, "", fmt.Errorf("cannot verify PR #%d: %s", prInfo.Index, result.Error)
	}

	return result.Merged, result.Title, nil
}

// validateTaskCompletion checks if a task can be completed.
// It blocks completion if the task has an unmerged PR.
// Returns the PR title if available (empty string if no PR).
// checker and resolver may be nil, in which case production defaults are used.
func validateTaskCompletion(
	modified hookTask, checker prMergedChecker, resolver pathResolver,
) (prTitle string, err error) {
	prID := modified.PRID()
	if prID == "" {
		return "", nil
	}

	prInfo, err := taskwarrior.ParsePRID(prID)
	if err != nil {
		return "", fmt.Errorf("cannot verify PR %q: %w", prID, err)
	}

	if resolver == nil {
		resolver = project.ResolveProjectPath
	}

	projectPath := resolver(modified.Project())
	if projectPath == "" {
		return "", fmt.Errorf("cannot verify PR: project %q not found in projects.toml. "+
			"Run `ttal project list` to see registered projects", modified.Project())
	}

	if checker == nil {
		checker = defaultPRMergedChecker
	}

	merged, title, err := checker(modified.Project(), projectPath, prID)
	if err != nil {
		return "", err
	}

	if !merged {
		return "", fmt.Errorf("cannot complete task with unmerged PR #%d. Merge the PR first", prInfo.Index)
	}

	return title, nil
}

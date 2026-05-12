package pairing

import (
	"strings"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// Manager returns the pair target for team-path manager-plane agent sessions.
func Manager(cfg *config.Config) string {
	if cfg == nil || cfg.AdminHuman == nil {
		return ""
	}
	return strings.TrimSpace(cfg.AdminHuman.Alias)
}

// PlanReviewer returns the pair target for the plan-review lead session.
func PlanReviewer(task *taskwarrior.Task) string {
	if task == nil {
		return ""
	}
	return strings.TrimSpace(task.Owner)
}

// Worker returns the pair target for a worker/reviewer window.
func Worker(cfg *config.Config, agentName string, task *taskwarrior.Task) string {
	if agentName == "coder" {
		return taskOwner(task)
	}
	return agentFrontmatter(cfg, agentName)
}

// Reviewer returns the pair target for a PR-reviewer window.
func Reviewer(cfg *config.Config, reviewerName string) string {
	if reviewerName == "pr-review-lead" {
		return "coder"
	}
	return agentFrontmatter(cfg, reviewerName)
}

func taskOwner(task *taskwarrior.Task) string {
	if task == nil {
		return ""
	}
	return strings.TrimSpace(task.Owner)
}

func agentFrontmatter(cfg *config.Config, agentName string) string {
	if cfg == nil {
		return ""
	}
	return agentfs.ResolvePairWith(cfg.TeamPath, cfg.Sync.WorkerAgentPaths, agentName)
}

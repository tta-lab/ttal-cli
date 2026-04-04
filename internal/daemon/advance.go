package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/gitutil"
	"github.com/tta-lab/ttal-cli/internal/notification"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/planreview"
	"github.com/tta-lab/ttal-cli/internal/pr"
	projectPkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/review"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/status"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// AdvanceRequest is the request body for POST /pipeline/advance.
type AdvanceRequest struct {
	TaskUUID    string `json:"task_uuid"`
	AgentName   string `json:"agent_name"`             // from TTAL_AGENT_NAME env in caller session
	Team        string `json:"team"`                   // TODO: remove after in-flight request compat window (~2026 Q3)
	SessionName string `json:"session_name,omitempty"` // caller's tmux session (for reviewer spawn)
	WorkDir     string `json:"work_dir,omitempty"`     // caller's cwd (for reviewer spawn)
}

// AdvanceResponse is the response body for POST /pipeline/advance.
type AdvanceResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Stage    string `json:"stage"`              // new stage name if advanced
	Reviewer string `json:"reviewer,omitempty"` // reviewer agent name if NeedsLGTM
	Assignee string `json:"assignee,omitempty"` // stage assignee role (e.g. "designer", "worker")
	Agent    string `json:"agent,omitempty"`    // resolved agent name (e.g. "mira", "kestrel")
}

// Advance status constants.
const (
	AdvanceStatusAdvanced   = "advanced"
	AdvanceStatusNeedsLGTM  = "needs_lgtm"
	AdvanceStatusRejected   = "rejected"
	AdvanceStatusError      = "error"
	AdvanceStatusNoPipeline = "no_pipeline"
	AdvanceStatusComplete   = "complete"
)

// AdvanceClient sends an advance request to the daemon and blocks until response.
func AdvanceClient(req AdvanceRequest) (AdvanceResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return AdvanceResponse{}, fmt.Errorf("encode request: %w", err)
	}

	client := daemonHTTPClientLong(askHumanClientTimeout)
	resp, err := client.Post(daemonBaseURL+"/pipeline/advance", "application/json", bytes.NewReader(body))
	if err != nil {
		return AdvanceResponse{}, fmt.Errorf("daemon not running: %w", err)
	}
	defer resp.Body.Close()

	var result AdvanceResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return AdvanceResponse{}, fmt.Errorf("invalid response from daemon (HTTP %d): %w", resp.StatusCode, err)
	}
	return result, nil
}

// handlePipelineAdvance is the daemon-side HTTP handler for POST /pipeline/advance.
// It may block for minutes when a "human" gate requires Telegram approval.
func handlePipelineAdvance(
	w http.ResponseWriter, r *http.Request,
	fe frontend.Frontend, mcfg *config.DaemonConfig,
	workerRuntime string,
) {
	var req AdvanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "invalid JSON: " + err.Error(),
		})
		return
	}

	team := mcfg.DefaultTeamName()
	teamPath := resolveTeamPath(mcfg, team)

	task, err := taskwarrior.ExportTask(req.TaskUUID)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: fmt.Sprintf("cannot fetch task %s: %v", req.TaskUUID, err),
		})
		return
	}

	if task.Status == "completed" {
		writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: fmt.Sprintf("task %s is already completed", req.TaskUUID),
		})
		return
	}

	p, ok := matchTaskPipeline(w, task.Tags)
	if !ok {
		return
	}

	agentRoles := buildAgentRoles(teamPath)

	// Only resolve current stage when task is active (started).
	// When not active, agent tags are routing hints, not stage assignments.
	idx := -1
	var stage *pipeline.Stage
	if task.Start != "" {
		var err error
		idx, stage, err = p.CurrentStage(task.Tags)
		if err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: "determine stage: " + err.Error(),
			})
			return
		}
	}

	if idx == -1 {
		// First advance — route to stage 0.
		firstStage := &p.Stages[0]
		startRecord := fmt.Sprintf("pipeline:started stage:%s completed:%s", firstStage.Name, nowUTC())
		if err := taskwarrior.AnnotateTask(task.UUID, startRecord); err != nil {
			log.Printf("[advance] warning: annotate pipeline start: %v", err)
		}
		err := advanceToStage(w, mcfg, task, firstStage, req.AgentName, team, workerRuntime, teamPath, agentRoles)
		if err != nil {
			log.Printf("[advance] first stage error: %v", err)
		}
		return
	}

	// Guard: reject manager-plane agents whose stage is already complete.
	if checkCallerPastStage(w, p, idx, req.AgentName, agentRoles, task.UUID, task.Tags) {
		return
	}

	// Guard: reject named manager-plane agents from advancing another agent's owned task.
	// Once a task is assigned to an agent (owner tag set), only that agent can drive it forward.
	// Workers (empty AgentName or unknown to agentRoles) always pass through.
	if checkOwnershipGuard(w, task, req.AgentName, agentRoles) {
		return
	}

	processStageAdvance(r.Context(), w, fe, mcfg, task, p, idx, stage,
		req.AgentName, req.SessionName, req.WorkDir, team, workerRuntime, teamPath, agentRoles)
}

// resolveTeamPath returns the filesystem path for the given team name.
func resolveTeamPath(mcfg *config.DaemonConfig, team string) string {
	resolvedTeam := mcfg.Teams[team]
	if resolvedTeam == nil {
		return ""
	}
	return resolvedTeam.TeamPath
}

// buildAgentRoles discovers agents from the team path and returns a name→role map.
func buildAgentRoles(teamPath string) map[string]string {
	agentRoles := make(map[string]string)
	if teamPath == "" {
		return agentRoles
	}
	agents, err := agentfs.Discover(teamPath)
	if err != nil {
		log.Printf("[advance] warning: discover agents in %s: %v", teamPath, err)
		return agentRoles
	}
	for _, a := range agents {
		agentRoles[a.Name] = a.Role
	}
	return agentRoles
}

// matchTaskPipeline loads pipeline config and matches it against the task tags.
// Returns (pipeline, true) on success; writes the HTTP error response and returns (nil, false) on failure.
func matchTaskPipeline(w http.ResponseWriter, taskTags []string) (*pipeline.Pipeline, bool) {
	configDir := config.DefaultConfigDir()
	pipelineCfg, err := pipeline.Load(configDir)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "load pipeline config: " + err.Error(),
		})
		return nil, false
	}

	_, p, err := pipelineCfg.MatchPipeline(taskTags)
	if err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "pipeline conflict: " + err.Error(),
		})
		return nil, false
	}
	if p == nil {
		msg := "no pipeline matches this task's tags — add a pipeline tag\n\nAvailable pipelines:\n" + pipelineCfg.Summary()
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusNoPipeline,
			Message: msg,
		})
		return nil, false
	}
	return p, true
}

// nowUTC returns the current UTC time formatted as RFC3339.
func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// annotateStageCompletion writes an audit annotation recording who completed the stage.
func annotateStageCompletion(uuid string, stage *pipeline.Stage, agentName string) {
	completedBy := agentName
	if stage.IsWorker() {
		completedBy = stage.Assignee
	} else if completedBy == "" {
		completedBy = "unknown"
	}
	record := fmt.Sprintf("stage:%s by:%s completed:%s", stage.Name, completedBy, nowUTC())
	if err := taskwarrior.AnnotateTask(uuid, record); err != nil {
		log.Printf("[advance] warning: annotate stage completion: %v", err)
	}
}

// processStageAdvance handles gate checks and advancement for an already-active stage.
func processStageAdvance(
	ctx context.Context,
	w http.ResponseWriter,
	fe frontend.Frontend,
	mcfg *config.DaemonConfig,
	task *taskwarrior.Task,
	p *pipeline.Pipeline,
	idx int,
	stage *pipeline.Stage,
	callerAgent, sessionName, workDir, team, workerRuntime, teamPath string,
	agentRoles map[string]string,
) {
	if stage.Reviewer != "" && !hasTag(task.Tags, stage.StageLGTMTag()) {
		// Attempt spawn/re-trigger before writing response so we can include the outcome.
		// Skip when sessionName is empty (old client or non-tmux caller) — backwards compatible.
		var spawnMsg string
		switch {
		case sessionName == "":
			// Old client or non-tmux caller — skip spawn for backwards compatibility.
		case mcfg.Global == nil:
			log.Printf("[advance] skipping reviewer spawn for task %s: global config not loaded", task.UUID)
			spawnMsg = " (spawn skipped: daemon config not loaded)"
		default:
			err := spawnOrRetriggerReviewerFromDaemon(task, stage, sessionName, workDir, team, agentRoles, mcfg.Global)
			if err != nil {
				log.Printf("[advance] warning: reviewer spawn failed for task %s: %v", task.UUID, err)
				spawnMsg = fmt.Sprintf(" (spawn failed: %v)", err)
			} else {
				spawnMsg = " (reviewer spawned)"
			}
		}

		msg := fmt.Sprintf("⏸ Waiting for reviewer (%s) verdict%s", stage.Reviewer, spawnMsg)
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:   AdvanceStatusNeedsLGTM,
			Message:  msg,
			Reviewer: stage.Reviewer,
			Assignee: stage.Assignee,
		})
		return
	}
	if checkHumanGate(ctx, w, fe, p, idx, callerAgent, task, stage) {
		return
	}

	cleanupAssigneeTags(task, stage, agentRoles)

	if stage.IsWorker() && task.PRID != "" {
		if done := handleWorkerPRMerge(w, task); done {
			return
		}
	}

	nextIdx := idx + 1
	if nextIdx >= len(p.Stages) {
		handlePipelineComplete(w, task, stage)
		return
	}

	err := advanceToStage(w, mcfg, task, &p.Stages[nextIdx], callerAgent, team, workerRuntime, teamPath, agentRoles)
	if err != nil {
		log.Printf("[advance] next stage error: %v", err)
	}
}

// resolveReviewerSession determines which tmux session should host the plan-review window.
// The reviewer belongs in the task owner's session (the agent whose name tag is on the task),
// not the caller's session (the agent who ran ttal go).
// Falls back to callerSession when no agent tag is found on the task or when team is empty.
func resolveReviewerSession(taskTags []string, agentRoles map[string]string, team, callerSession string) string {
	if team == "" {
		return callerSession
	}
	ownerAgent := findAgentTag(taskTags, agentRoles)
	if ownerAgent == "" {
		return callerSession
	}
	return config.AgentSessionName(team, ownerAgent)
}

// spawnOrRetriggerReviewerFromDaemon spawns or re-triggers a reviewer from the daemon process.
// workDir is the caller's working directory (passed via AdvanceRequest).
// For plan-review, workDir is overridden with the project's registered path so the reviewer
// runs in the correct directory, not the caller's workspace.
// For PR-review, workDir is used as-is — the caller is the worker.
func spawnOrRetriggerReviewerFromDaemon(
	task *taskwarrior.Task, stage *pipeline.Stage,
	sessionName, workDir, team string,
	agentRoles map[string]string,
	cfg *config.Config,
) error {
	reviewerAgent := stage.Reviewer

	if stage.IsWorker() {
		if tmux.WindowExists(sessionName, reviewerAgent) {
			log.Printf("[advance] re-triggering PR reviewer %s for task %s", reviewerAgent, task.UUID)
			return review.RequestReReview(sessionName, reviewerAgent, false, "", cfg)
		}
		log.Printf("[advance] spawning PR reviewer %s for task %s", reviewerAgent, task.UUID)
		ctx, err := buildPRContextFromTask(task, workDir)
		if err != nil {
			return fmt.Errorf("build PR context: %w", err)
		}
		return review.SpawnReviewer(sessionName, ctx, reviewerAgent, cfg, workDir)
	}

	// Plan-review: resolve the task owner's session instead of using the caller's.
	// PR-review (above) correctly uses the caller's session — the caller is the worker.
	targetSession := resolveReviewerSession(task.Tags, agentRoles, team, sessionName)
	targetWorkDir := workDir
	if projectPath := projectPkg.ResolveProjectPath(task.Project); projectPath != "" {
		targetWorkDir = projectPath
	}

	if tmux.WindowExists(targetSession, reviewerAgent) {
		log.Printf("[advance] re-triggering plan reviewer %s for task %s in session %q",
			reviewerAgent, task.UUID, targetSession)
		return planreview.RequestReReview(targetSession, reviewerAgent, "", cfg)
	}
	log.Printf("[advance] spawning plan reviewer %s for task %s in session %q",
		reviewerAgent, task.UUID, targetSession)
	return planreview.SpawnPlanReviewer(targetSession, task, reviewerAgent, cfg, targetWorkDir)
}

// buildPRContextFromTask builds a PR context from a task and working directory.
// Used by the daemon when spawning a PR reviewer.
func buildPRContextFromTask(task *taskwarrior.Task, workDir string) (*pr.Context, error) {
	projectPath := projectPkg.ResolveProjectPath(task.Project)
	if projectPath == "" && workDir != "" {
		projectPath = workDir
	}
	if projectPath == "" {
		return nil, fmt.Errorf("cannot resolve project path for %q", task.Project)
	}
	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return nil, fmt.Errorf("detect git provider from %s: %w", projectPath, err)
	}
	return &pr.Context{
		Task:  task,
		Owner: info.Owner,
		Repo:  info.Repo,
		Info:  info,
	}, nil
}

// checkCallerPastStage rejects manager-plane agents whose role belongs to an earlier
// pipeline stage than the task's current stage. Workers (empty callerAgent) and agents
// whose role is not in the pipeline (e.g., orchestrators) are allowed through.
// Skips the guard when the current stage already has its _lgtm tag — that means the
// pipeline is fully completed and processStageAdvance should handle completion.
// Returns true when the response has been written (caller should return).
func checkCallerPastStage(
	w http.ResponseWriter,
	p *pipeline.Pipeline,
	currentIdx int,
	callerAgent string,
	agentRoles map[string]string,
	taskUUID string,
	taskTags []string,
) bool {
	if callerAgent == "" {
		return false
	}
	callerRole, ok := agentRoles[callerAgent]
	if !ok {
		return false
	}
	callerIdx := p.StageIndexForRole(callerRole)
	if callerIdx < 0 {
		return false
	}
	if callerIdx >= currentIdx {
		return false
	}
	// The current stage is already approved — let processStageAdvance → handlePipelineComplete handle it.
	if hasTag(taskTags, p.Stages[currentIdx].StageLGTMTag()) {
		return false
	}
	callerStageName := p.Stages[callerIdx].Name
	currentStageName := p.Stages[currentIdx].Name
	writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
		Status: AdvanceStatusRejected,
		Message: fmt.Sprintf("Task %s is already at stage %s — your stage (%s) is complete. No action needed.",
			taskUUID, currentStageName, callerStageName),
	})
	return true
}

// checkOwnershipGuard rejects named manager-plane agents from advancing a task
// owned by a different agent. Once a task has an owner tag (set by advanceToStage when
// routing to an agent), only that agent may drive it forward. Workers (empty AgentName
// or not found in agentRoles) and unowned tasks always pass through.
// Returns true when the response has been written (caller should return).
func checkOwnershipGuard(
	w http.ResponseWriter,
	task *taskwarrior.Task,
	callerAgent string,
	agentRoles map[string]string,
) bool {
	if callerAgent == "" {
		return false
	}
	if _, isAgent := agentRoles[callerAgent]; !isAgent {
		return false // not a recognized manager-plane agent (e.g. worker session name)
	}
	ownerAgent := findAgentTag(task.Tags, agentRoles)
	if ownerAgent == "" || ownerAgent == callerAgent {
		return false
	}
	writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
		Status: AdvanceStatusRejected,
		Message: fmt.Sprintf("Task %s is owned by %s — only they can advance it.",
			task.HexID(), ownerAgent),
	})
	return true
}

// checkHumanGate prompts for human approval when the stage has a human gate.
// Returns true when the response has been written (caller should return).
func checkHumanGate(
	ctx context.Context, w http.ResponseWriter, fe frontend.Frontend,
	p *pipeline.Pipeline, idx int, callerAgent string,
	task *taskwarrior.Task, stage *pipeline.Stage,
) bool {
	if stage.Gate != "human" || callerAgent == "" {
		return false
	}
	nextStageName := "Complete"
	if idx+1 < len(p.Stages) {
		nextStageName = p.Stages[idx+1].Name
	}
	approved, err := askHumanGate(ctx, fe, callerAgent, task, nextStageName)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "gate error: " + err.Error(),
		})
		return true
	}
	if !approved {
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusRejected,
			Message: "advance rejected by human",
		})
		return true
	}
	return false
}

// cleanupAssigneeTags removes the agent assignee tag to free the agent for other tasks.
// Stage tags are monotonic (never removed). Task is NOT stopped — it stays active
// throughout the pipeline lifecycle. Worker stages have no agent tag to remove.
func cleanupAssigneeTags(task *taskwarrior.Task, stage *pipeline.Stage, agentRoles map[string]string) {
	oldAgentName := findAgentTag(task.Tags, agentRoles)
	annotateStageCompletion(task.UUID, stage, oldAgentName)

	if oldAgentName != "" {
		if err := taskwarrior.ModifyTags(task.UUID, "-"+oldAgentName); err != nil {
			log.Printf("[advance] warning: remove agent tag: %v", err)
		}
	}
}

// handlePipelineComplete writes the pipeline-complete response.
// For worker+PR stages the cleanup handler owns MarkDone; for others it marks done inline.
func handlePipelineComplete(w http.ResponseWriter, task *taskwarrior.Task, stage *pipeline.Stage) {
	if stage.IsWorker() && task.PRID != "" {
		// Worker+PR: cleanup handler calls MarkDone after session teardown.
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusComplete,
			Message: "pipeline complete — cleanup in progress",
		})
		return
	}
	if err := taskwarrior.MarkDone(task.UUID); err != nil {
		log.Printf("[advance] warning: mark done: %v", err)
	}
	writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
		Status:  AdvanceStatusComplete,
		Message: "pipeline complete — task marked done",
	})
}

// shouldBreatheStatus is the pure logic: returns true when the agent should be breathed.
// Stale (>5min) or nil status defaults to true (breathe when uncertain).
func shouldBreatheStatus(agentStatus *status.AgentStatus, threshold float64) bool {
	if agentStatus == nil || agentStatus.IsStale(5*time.Minute) {
		return true
	}
	return agentStatus.ContextUsedPct >= threshold
}

// shouldBreathe reads the agent's status file and decides whether to breathe.
func shouldBreathe(team, agentName string, threshold float64) bool {
	agentStatus, err := status.ReadAgent(team, agentName)
	if err != nil {
		log.Printf("[advance] warning: could not read status for %s/%s, defaulting to breathe: %v", team, agentName, err)
		return true
	}
	return shouldBreatheStatus(agentStatus, threshold)
}

// countTasksFn is the function used to count active tasks. Package-level var for test injection.
var countTasksFn = taskwarrior.CountTasks

// worktreePathFn is the function used to resolve the worktree directory for a task.
// Package-level var for test injection.
var worktreePathFn = worker.WorktreePath

// notifyTelegramFn is the function used to send notifications.
// Package-level var for test injection. Default is set during daemon init via
// SetNotifyFn to close over the default team frontend.
// Before daemon init, falls back to worker.NotifyTelegram (e.g. in tests).
var notifyTelegramFn = worker.NotifyTelegram

// SetNotifyFn replaces the default notifyTelegramFn with a frontend-backed
// implementation. Called by daemon.Run() after frontends are built.
func SetNotifyFn(fn func(string)) {
	notifyTelegramFn = fn
}

// resolveHintedAgent checks task tags for a routing hint — a tag matching a known
// agent name with the required role. Returns the agent if found and idle, nil otherwise.
//
// Return contract: nil means "no usable hint" — caller MUST fall back to findIdleAgent.
// Busy agents return nil (soft fallback with log warning), never an error.
//
// If multiple tags match agents with the required role, the first match wins.
// Tag ordering in taskwarrior is not guaranteed, so tasks should have at most
// one agent hint tag per role.
func resolveHintedAgent(
	teamPath string, taskTags []string, requiredRole string, agentRoles map[string]string,
) *agentfs.AgentInfo {
	for _, tag := range taskTags {
		role, isAgent := agentRoles[tag]
		if !isAgent || role != requiredRole {
			continue
		}
		// Found a hint tag — resolve full agent info.
		agent, err := agentfs.Get(teamPath, tag)
		if err != nil {
			log.Printf("[advance] warning: resolve hinted agent %s: %v", tag, err)
			return nil
		}
		// Check idle/busy.
		count, err := countTasksFn(fmt.Sprintf("+%s", tag), "+ACTIVE")
		if err != nil {
			log.Printf("[advance] warning: check idle for hinted agent %s: %v", tag, err)
			return nil
		}
		if count > 0 {
			log.Printf("[advance] hinted agent %s is busy (%d active tasks), falling back to role-based routing", tag, count)
			return nil
		}
		log.Printf("[advance] routing to hinted agent %s (role: %s)", tag, requiredRole)
		return agent
	}
	return nil
}

// resolveStageAgent returns the agent to route to: hinted agent if idle, else any idle agent with the role.
func resolveStageAgent(
	teamPath string, taskTags []string, assignee string, agentRoles map[string]string,
) (*agentfs.AgentInfo, error) {
	if hinted := resolveHintedAgent(teamPath, taskTags, assignee, agentRoles); hinted != nil {
		return hinted, nil
	}
	return findIdleAgent(teamPath, assignee)
}

// isWorkerStage returns true if the stage should be handled as a worker spawn.
// It returns true if stage.Worker is explicitly set, OR if the stage assignee
// is not a known agent role — guarding against pipelines where worker=true was
// omitted but the assignee is a CC agent name (e.g. "coder"), not a role.
func isWorkerStage(stage *pipeline.Stage, agentRoles map[string]string) bool {
	if stage.IsWorker() {
		return true
	}
	for _, role := range agentRoles {
		if role == stage.Assignee {
			return false // assignee is a valid role — route to an agent
		}
	}
	// Assignee not found as any agent's role → treat as worker agent name.
	log.Printf("[advance] stage %q assignee %q is not a known role — treating as worker stage", stage.Name, stage.Assignee)
	return true
}

// advanceToStage routes the task to the given stage (agent or worker).
func advanceToStage(
	w http.ResponseWriter,
	mcfg *config.DaemonConfig,
	task *taskwarrior.Task,
	stage *pipeline.Stage,
	callerAgent, team, workerRuntime string,
	teamPath string,
	agentRoles map[string]string,
) error {
	if isWorkerStage(stage, agentRoles) {
		// Worker stage: start task and spawn.
		if err := taskwarrior.StartTask(task.UUID); err != nil {
			log.Printf("[advance] warning: start task: %v", err)
		}
		if err := taskwarrior.ModifyTags(task.UUID, "+"+stage.StageTag()); err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: fmt.Sprintf("add stage tag: %v", err),
			})
			return err
		}

		projectPath, err := projectPkg.ResolveProjectPathOrError(task.Project)
		if err != nil {
			writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: fmt.Sprintf("resolve project %q: %v", task.Project, err),
			})
			return err
		}

		spawner := team + ":" + callerAgent
		if callerAgent == "" {
			spawner = ""
		}

		spawnCfg := worker.SpawnConfig{
			Name:      task.HexID(),
			Project:   projectPath,
			TaskUUID:  task.UUID,
			Worktree:  true,
			Runtime:   runtime.Runtime(workerRuntime), //nolint:unconvert
			Spawner:   spawner,
			AgentName: stage.Assignee,
		}

		if err := worker.Spawn(spawnCfg); err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: "spawn worker: " + err.Error(),
			})
			return err
		}

		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:   AdvanceStatusAdvanced,
			Stage:    stage.Name,
			Assignee: stage.Assignee,
		})
		return nil
	}

	// Agent stage: check for routing hint, then fall back to role-based routing.
	agent, err := resolveStageAgent(teamPath, task.Tags, stage.Assignee, agentRoles)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: fmt.Sprintf("find idle agent for role %q: %v", stage.Assignee, err),
		})
		return err
	}

	if err := taskwarrior.ModifyTags(task.UUID, "+"+agent.Name); err != nil {
		log.Printf("[advance] warning: add agent tag: %v", err)
	}
	if err := taskwarrior.ModifyTags(task.UUID, "+"+stage.StageTag()); err != nil {
		log.Printf("[advance] warning: add stage tag: %v", err)
	}
	if err := taskwarrior.StartTask(task.UUID); err != nil {
		log.Printf("[advance] warning: start task for agent: %v", err)
	}

	cfg := mcfg.Global
	agentRT := cfg.AgentRuntimeFor(agent.Name)
	rolePrompt := cfg.RenderPrompt(agent.Role, task.UUID, agentRT)
	rolePrompt = pipeline.PrependSkills(rolePrompt, stage.Skills, agentRT)

	if err := routeToPersistentAgent(w, cfg, task, agent, rolePrompt, callerAgent, team); err != nil {
		return err
	}

	record := fmt.Sprintf("advanced: %s → %s (stage: %s)", callerAgent, agent.Name, stage.Name)
	if err := taskwarrior.AnnotateTask(task.UUID, record); err != nil {
		log.Printf("[advance] warning: annotate task: %v", err)
	}

	writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
		Status:   AdvanceStatusAdvanced,
		Stage:    stage.Name,
		Assignee: stage.Assignee,
		Agent:    agent.Name,
	})
	return nil
}

// askHumanGate sends a Telegram approval request and blocks until answered.
// Returns true if approved, false if rejected or timed out.
func askHumanGate(
	ctx context.Context, fe frontend.Frontend, agentName string,
	task *taskwarrior.Task, nextStageName string,
) (bool, error) {
	question := notification.GateRequest{
		Ctx: notification.NewContext(task.Project, task.HexID(), task.Description, nextStageName),
	}.RenderHTML()
	options := []string{frontend.GateOptionApprove, frontend.GateOptionReject}
	answer, skipped, err := fe.AskHuman(ctx, agentName, question, options)
	if err != nil {
		return false, err
	}
	if skipped {
		return false, nil
	}
	return answer == frontend.GateOptionApprove, nil
}

// findIdleAgent finds the first idle agent with the given role.
// An agent is idle if they have no started pending tasks with their name as a tag.
func findIdleAgent(teamPath, role string) (*agentfs.AgentInfo, error) {
	agents, err := agentfs.FindByRole(teamPath, role)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agent with role %q found", role)
	}

	var queryErrors []string
	for i := range agents {
		count, err := taskwarrior.CountTasks(fmt.Sprintf("+%s", agents[i].Name), "+ACTIVE")
		if err != nil {
			queryErrors = append(queryErrors, fmt.Sprintf("%s: %v", agents[i].Name, err))
			continue
		}
		if count == 0 {
			return &agents[i], nil
		}
	}

	names := make([]string, len(agents))
	for i, a := range agents {
		names[i] = a.Name
	}
	if len(queryErrors) > 0 {
		return nil, fmt.Errorf("taskwarrior query failed for role %q agents: %v", role, queryErrors)
	}
	return nil, fmt.Errorf("all agents with role %q are busy: %v", role, names)
}

// routeToPersistentAgent optionally breathes a persistent agent.
// The route file mechanism has been removed — taskwarrior state (stage tag) is the SSOT.
// When the agent breathes, ttal context renders the universal context template, and
// $ ttal pipeline prompt reads the stage tag to output the role-specific prompt.
func routeToPersistentAgent(
	w http.ResponseWriter, cfg *config.Config,
	_ *taskwarrior.Task, agent *agentfs.AgentInfo,
	_, _, team string,
) error {
	if !shouldBreathe(team, agent.Name, cfg.BreatheThreshold()) {
		log.Printf("[advance] skipping breathe for %s (ctx below %.0f%% threshold)", agent.Name, cfg.BreatheThreshold())
		return nil
	}
	if err := Send(SendRequest{From: "system", To: agent.Name, Message: "run ttal skill get breathe\n\nExecute this skill now — your context window needs a refresh."}); err != nil { //nolint:lll
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: fmt.Sprintf("send breathe to %s: %v", agent.Name, err),
		})
		return err
	}
	return nil
}

// hasTag reports whether the given tag is present in the tags slice.
func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// findAgentTag returns the first tag that matches a known agent name.
func findAgentTag(tags []string, agentRoles map[string]string) string {
	for _, t := range tags {
		if _, ok := agentRoles[t]; ok {
			return t
		}
	}
	return ""
}

// handleWorkerPRMerge merges the worker PR and requests cleanup.
// Returns true when the HTTP response has been written (caller should return).
func handleWorkerPRMerge(w http.ResponseWriter, task *taskwarrior.Task) bool {
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		log.Printf("[advance] warning: could not load config, defaulting to auto-merge: %v", cfgErr)
	}
	if cfgErr == nil && cfg.GetMergeMode() == config.MergeModeManual {
		notifyTelegramFn(notification.PRReadyToMerge{
			Ctx: notification.NewContext(task.Project, task.HexID(), task.Description, ""),
		}.Render())
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusNeedsLGTM,
			Message: "Manual merge mode — PR ready for human merge",
		})
		return true
	}

	// Block merge if worktree has uncommitted changes.
	if worktreeDir, err := worktreePathFn(task.UUID, task.Project); err == nil {
		if _, statErr := os.Stat(worktreeDir); os.IsNotExist(statErr) {
			// Worktree already removed — skip guard, let merge proceed.
			log.Printf("[advance] worktree absent, skipping dirty check: %s", worktreeDir)
		} else if clean, gitErr := gitutil.IsWorktreeClean(worktreeDir); gitErr != nil {
			// Directory exists but git status failed (locked repo, timeout, etc.) — block to be safe.
			msg := fmt.Sprintf("dirty check failed for worktree %s: %v", worktreeDir, gitErr)
			log.Printf("[advance] blocked merge: %s", msg)
			notifyTelegramFn(notification.PRMergeBlocked{
				Ctx:    notification.NewContext(task.Project, task.HexID(), task.Description, ""),
				Reason: "could not verify worktree state",
			}.Render())
			writeHTTPJSON(w, http.StatusConflict, AdvanceResponse{
				Status:  AdvanceStatusRejected,
				Message: msg,
			})
			return true
		} else if !clean {
			msg := fmt.Sprintf("worktree has uncommitted changes — commit or discard before merging (%s)", worktreeDir)
			log.Printf("[advance] blocked merge: %s", msg)
			notifyTelegramFn(notification.PRMergeBlocked{
				Ctx:    notification.NewContext(task.Project, task.HexID(), task.Description, ""),
				Reason: "uncommitted changes in worktree",
			}.Render())
			writeHTTPJSON(w, http.StatusConflict, AdvanceResponse{
				Status:  AdvanceStatusRejected,
				Message: msg,
			})
			return true
		}
	}

	if err := mergeWorkerPR(task); err != nil {
		log.Printf("[advance] PR merge failed: %v", err)
		notifyTelegramFn(notification.PRMergeFailed{
			Ctx:    notification.NewContext(task.Project, task.HexID(), task.Description, ""),
			Reason: err.Error(),
		}.Render())
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "merge PR: " + err.Error(),
		})
		return true
	}

	if err := worker.RequestCleanup(task.SessionName(), task.UUID); err != nil {
		log.Printf("[advance] warning: cleanup request failed: %v", err)
	}
	return false
}

// mergeWorkerPR merges the PR associated with the task.
// Delegates to handlePRMerge (a plain function call, not an HTTP loopback).
func mergeWorkerPR(task *taskwarrior.Task) error {
	projectPath := projectPkg.ResolveProjectPath(task.Project)
	if projectPath == "" {
		return fmt.Errorf("cannot resolve project path for %q", task.Project)
	}

	info, err := gitprovider.DetectProvider(projectPath)
	if err != nil {
		return fmt.Errorf("detect git provider: %w", err)
	}

	prInfo, err := taskwarrior.ParsePRID(task.PRID)
	if err != nil {
		return fmt.Errorf("parse pr_id: %w", err)
	}

	resp := handlePRMerge(PRMergeRequest{
		ProviderType: string(info.Provider),
		Owner:        info.Owner,
		Repo:         info.Repo,
		Index:        prInfo.Index,
		DeleteBranch: true,
		ProjectAlias: task.Project,
	})

	if !resp.OK {
		// handlePRMerge treats "already merged" as an error, but for
		// pipeline advancement it's a no-op success.
		if resp.AlreadyMerged {
			log.Printf("[advance] PR #%d already merged, skipping", prInfo.Index)
			return nil
		}
		return errors.New(resp.Error)
	}

	log.Printf("[advance] PR #%d merged (squash)", prInfo.Index)
	return nil
}

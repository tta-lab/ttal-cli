package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"strings"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	projectPkg "github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/route"
	"github.com/tta-lab/ttal-cli/internal/runtime"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// AdvanceRequest is the request body for POST /pipeline/advance.
type AdvanceRequest struct {
	TaskUUID  string `json:"task_uuid"`
	AgentName string `json:"agent_name"` // from TTAL_AGENT_NAME env in caller session
	Team      string `json:"team"`       // from TTAL_TEAM env in caller session
}

// AdvanceResponse is the response body for POST /pipeline/advance.
type AdvanceResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Stage   string `json:"stage"` // new stage name if advanced
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
	resp, err := client.Post(daemonBaseURL+"/pipeline/advance", "application/json", strings.NewReader(string(body)))
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

	// Resolve team — fall back to default if caller didn't set TTAL_TEAM.
	team := req.Team
	if team == "" {
		team = mcfg.DefaultTeamName()
	}

	resolvedTeam := mcfg.Teams[team]
	teamPath := ""
	if resolvedTeam != nil {
		teamPath = resolvedTeam.TeamPath
	}

	// Export task.
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

	// Load pipeline config.
	configDir := config.DefaultConfigDir()
	pipelineCfg, err := pipeline.Load(configDir)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "load pipeline config: " + err.Error(),
		})
		return
	}

	// Match task to pipeline.
	_, p, err := pipelineCfg.MatchPipeline(task.Tags)
	if err != nil {
		writeHTTPJSON(w, http.StatusBadRequest, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "pipeline conflict: " + err.Error(),
		})
		return
	}
	if p == nil {
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusNoPipeline,
			Message: "no pipeline matches this task's tags — add a pipeline tag (e.g. +feature)",
		})
		return
	}

	// Build agent roles map.
	agentRoles := make(map[string]string)
	if teamPath != "" {
		agents, _ := agentfs.Discover(teamPath)
		for _, a := range agents {
			agentRoles[a.Name] = a.Role
		}
	}

	// Determine current stage.
	idx, stage, err := p.CurrentStage(task.Tags, agentRoles)
	if err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "determine stage: " + err.Error(),
		})
		return
	}

	if idx == -1 {
		// First advance — route to stage 0.
		firstStage := &p.Stages[0]
		if err := advanceToStage(r.Context(), w, fe, mcfg, task, 0, firstStage, req.AgentName, team, workerRuntime, teamPath, agentRoles); err != nil {
			log.Printf("[advance] first stage error: %v", err)
		}
		return
	}

	// Stage is active — check reviewer gate before advancing.
	if stage.Reviewer != "" && !hasTag(task.Tags, "lgtm") {
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusNeedsLGTM,
			Message: fmt.Sprintf("Run reviewer (%s) and set verdict with: ttal task comment <uuid> \"message\" --verdict lgtm", stage.Reviewer),
		})
		return
	}

	// Check human gate.
	if stage.Gate == "human" {
		approved, err := askHumanGate(r.Context(), fe, req.AgentName, task, stage)
		if err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: "gate error: " + err.Error(),
			})
			return
		}
		if !approved {
			writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
				Status:  AdvanceStatusRejected,
				Message: "advance rejected by human",
			})
			return
		}
	}

	// Advance to next stage.
	if err := taskwarrior.StopTask(task.UUID); err != nil {
		log.Printf("[advance] warning: stop task: %v", err)
	}

	// Remove old agent tag and lgtm tag.
	oldAgentName := findAgentTag(task.Tags, agentRoles)
	removeTags := []string{"-lgtm"}
	if oldAgentName != "" {
		removeTags = append(removeTags, "-"+oldAgentName)
	}
	if err := taskwarrior.ModifyTags(task.UUID, removeTags...); err != nil {
		log.Printf("[advance] warning: remove tags: %v", err)
	}

	nextIdx := idx + 1
	if nextIdx >= len(p.Stages) {
		// Pipeline complete.
		if err := taskwarrior.MarkDone(task.UUID); err != nil {
			log.Printf("[advance] warning: mark done: %v", err)
		}
		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status:  AdvanceStatusComplete,
			Message: "pipeline complete — task marked done",
		})
		return
	}

	nextStage := &p.Stages[nextIdx]
	if err := advanceToStage(r.Context(), w, fe, mcfg, task, nextIdx, nextStage, req.AgentName, team, workerRuntime, teamPath, agentRoles); err != nil {
		log.Printf("[advance] next stage error: %v", err)
	}
}

// advanceToStage routes the task to the given stage (agent or worker).
func advanceToStage(
	ctx context.Context,
	w http.ResponseWriter,
	fe frontend.Frontend,
	mcfg *config.DaemonConfig,
	task *taskwarrior.Task,
	idx int,
	stage *pipeline.Stage,
	callerAgent, team, workerRuntime string,
	teamPath string,
	agentRoles map[string]string,
) error {
	if stage.Assignee == "worker" {
		// Worker stage: start task and spawn.
		if err := taskwarrior.StartTask(task.UUID); err != nil {
			log.Printf("[advance] warning: start task: %v", err)
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
			Name:     task.SessionID(),
			Project:  projectPath,
			TaskUUID: task.UUID,
			Worktree: true,
			Runtime:  runtime.Runtime(workerRuntime), //nolint:unconvert
			Spawner:  spawner,
		}

		if err := worker.Spawn(spawnCfg); err != nil {
			writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
				Status:  AdvanceStatusError,
				Message: "spawn worker: " + err.Error(),
			})
			return err
		}

		writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
			Status: AdvanceStatusAdvanced,
			Stage:  stage.Name,
		})
		return nil
	}

	// Agent stage: find idle agent with the required role and route to them.
	agent, err := findIdleAgent(teamPath, stage.Assignee, agentRoles)
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
	if err := taskwarrior.StartTask(task.UUID); err != nil {
		log.Printf("[advance] warning: start task for agent: %v", err)
	}

	// Build role prompt and route to agent.
	cfg := mcfg.Global
	agentRT := cfg.AgentRuntimeFor(agent.Name)
	rolePrompt := cfg.RenderPrompt(agent.Role, task.UUID, agentRT)
	trigger := fmt.Sprintf("New task routed to you: %s\nTask UUID: %s\nRun: ttal task get %s",
		task.Description, task.UUID[:8], task.UUID[:8])

	projectPath := projectPkg.ResolveProjectPath(task.Project)
	if err := route.Stage(agent.Name, route.Request{
		TaskUUID:    task.UUID,
		RolePrompt:  rolePrompt,
		Trigger:     trigger,
		ProjectPath: projectPath,
		RoutedBy:    callerAgent,
		Team:        team,
	}); err != nil {
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: "stage route: " + err.Error(),
		})
		return err
	}

	breatheMsg := "/breathe"
	if callerAgent != "" {
		breatheMsg = fmt.Sprintf("[agent from:%s] /breathe", callerAgent)
	}
	if err := Send(SendRequest{
		From:    callerAgent,
		To:      agent.Name,
		Message: breatheMsg,
	}); err != nil {
		// Cleanup route file on failure.
		if _, consumeErr := route.Consume(agent.Name); consumeErr != nil {
			log.Printf("[advance] warning: clean up route file for %s: %v", agent.Name, consumeErr)
		}
		writeHTTPJSON(w, http.StatusInternalServerError, AdvanceResponse{
			Status:  AdvanceStatusError,
			Message: fmt.Sprintf("send breathe to %s: %v", agent.Name, err),
		})
		return err
	}

	record := fmt.Sprintf("advanced: %s → %s (stage: %s)", callerAgent, agent.Name, stage.Name)
	if err := taskwarrior.AnnotateTask(task.UUID, record); err != nil {
		log.Printf("[advance] warning: annotate task: %v", err)
	}

	writeHTTPJSON(w, http.StatusOK, AdvanceResponse{
		Status: AdvanceStatusAdvanced,
		Stage:  stage.Name,
	})
	return nil
}

// askHumanGate sends a Telegram approval request and blocks until answered.
// Returns true if approved, false if rejected or timed out.
func askHumanGate(ctx context.Context, fe frontend.Frontend, agentName string, task *taskwarrior.Task, stage *pipeline.Stage) (bool, error) {
	question := fmt.Sprintf("🔒 Advance task through <b>%s</b> gate?\n\n📋 Task: %s\n🎯 Stage: %s",
		stage.Gate,
		html.EscapeString(task.Description),
		html.EscapeString(stage.Name),
	)
	options := []string{"✅ Approve", "❌ Reject"}
	answer, skipped, err := fe.AskHuman(ctx, agentName, question, options)
	if err != nil {
		return false, err
	}
	if skipped {
		return false, nil
	}
	return answer == "✅ Approve", nil
}

// findIdleAgent finds the first idle agent with the given role.
// An agent is idle if they have no started pending tasks with their name as a tag.
func findIdleAgent(teamPath, role string, agentRoles map[string]string) (*agentfs.AgentInfo, error) {
	agents, err := agentfs.FindByRole(teamPath, role)
	if err != nil {
		return nil, err
	}
	if len(agents) == 0 {
		return nil, fmt.Errorf("no agent with role %q found", role)
	}

	for i := range agents {
		count, err := taskwarrior.CountTasks(fmt.Sprintf("+%s", agents[i].Name), "+ACTIVE")
		if err != nil {
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
	return nil, fmt.Errorf("all agents with role %q are busy: %v", role, names)
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

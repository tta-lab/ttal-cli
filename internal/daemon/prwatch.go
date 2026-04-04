package daemon

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/notification"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/project"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

const (
	prScanInterval = 30 * time.Second
	prPollInitial  = 20 * time.Second
	prPollMax      = 2 * time.Minute
	prPollBackoff  = 1.5
	prWatchTimeout = 1 * time.Hour
)

// prWatchTarget holds derived info for a PR being polled.
type prWatchTarget struct {
	TaskUUID     string
	SessionName  string
	WindowName   string // tmux window name within the worker session (derived from pipeline assignee)
	Team         string
	Owner        string
	Repo         string
	PRIndex      int64
	Description  string
	Provider     string
	Spawner      string
	ProjectAlias string
}

// startPRWatcher periodically scans taskwarrior for pending tasks with pr_id set
// and active tmux sessions, then spawns per-PR polling goroutines.
func startPRWatcher(mcfg *config.DaemonConfig, frontends map[string]frontend.Frontend, done <-chan struct{}) {
	var mu sync.Mutex
	active := make(map[string]bool) // task UUID → polling

	go func() {
		// Initial scan after short delay (let daemon finish startup)
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()

		for {
			select {
			case <-done:
				return
			case <-timer.C:
			}

			scanForPRTasks(mcfg, frontends, &mu, active, done)
			timer.Reset(prScanInterval)
		}
	}()

	log.Printf("[prwatch] scanner started (interval=%s)", prScanInterval)
}

// scanForPRTasks queries taskwarrior for tasks with pr_id, checks tmux sessions,
// and spawns polling goroutines for new PRs.
func scanForPRTasks(
	mcfg *config.DaemonConfig, frontends map[string]frontend.Frontend,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) {
	teamName := mcfg.DefaultTeamName()
	seen := scanTeam(frontends, teamName, mu, active, done)
	if seen == nil {
		// Scan failed — skip pruning this round to avoid orphaning
		// goroutines whose UUIDs would be absent from an error-empty result.
		return
	}

	// Prune UUIDs from active that no longer appear in the task list.
	// These are tasks that were merged/closed and have since been marked done.
	mu.Lock()
	for uuid := range active {
		if !seen[uuid] {
			delete(active, uuid)
		}
	}
	mu.Unlock()
}

func scanTeam(
	frontends map[string]frontend.Frontend,
	teamName string,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) map[string]bool {
	tasks, err := taskwarrior.ListTasksWithPR()
	if err != nil {
		log.Printf("[prwatch] failed to list PR tasks for team %s: %v", teamName, err)
		return nil // nil signals caller to skip pruning pass
	}

	seenUUIDs := make(map[string]bool)

	for _, task := range tasks {
		seenUUIDs[task.UUID] = true

		mu.Lock()
		alreadyPolling := active[task.UUID]
		mu.Unlock()
		if alreadyPolling {
			continue
		}

		sessionName := task.SessionName()
		if !tmux.SessionExists(sessionName) {
			continue
		}

		prInfo, err := taskwarrior.ParsePRID(task.PRID)
		if err != nil {
			log.Printf("[prwatch] task %s has invalid pr_id %q: %v — skipping",
				task.UUID, task.PRID, err)
			continue
		}
		prIndex := prInfo.Index

		// Detect provider from project path
		projectPath := project.ResolveProjectPath(task.Project)
		if projectPath == "" {
			log.Printf("[prwatch] task %s: project %q not found in projects.toml — skipping PR watch",
				shortSHA(task.UUID), task.Project)
			continue
		}
		info, err := gitprovider.DetectProvider(projectPath)
		if err != nil {
			log.Printf("[prwatch] cannot detect provider for task %s: %v",
				shortSHA(task.UUID), err)
			continue
		}

		windowName := resolveWorkerWindowName(task.Tags)
		target := prWatchTarget{
			TaskUUID:     task.UUID,
			SessionName:  sessionName,
			WindowName:   windowName,
			Team:         teamName,
			Owner:        info.Owner,
			Repo:         info.Repo,
			PRIndex:      prIndex,
			Description:  task.Description,
			Provider:     string(info.Provider),
			Spawner:      task.Spawner,
			ProjectAlias: task.Project,
		}

		mu.Lock()
		active[task.UUID] = true
		mu.Unlock()

		log.Printf("[prwatch] starting poll: PR #%d %s/%s session=%s",
			prIndex, info.Owner, info.Repo, sessionName)

		go func() {
			keep := pollPR(target, frontends, done)
			if !keep {
				mu.Lock()
				delete(active, target.TaskUUID)
				mu.Unlock()
			}
			// If keep=true, UUID stays in active until task is no longer
			// returned by ListTasksWithPR (i.e. marked done), preventing re-spawn.
		}()
	}

	return seenUUIDs
}

// pollPR polls a PR's CI status until checks resolve, PR is merged/closed, or timeout.
// Delivers status exactly once per HEAD SHA.
// Returns true if the UUID should remain in the active map (PR merged/closed — wait for cleanup).
// Returns false for all other exits (timeout, session gone, shutdown) — allows re-spawn.
func pollPR(
	target prWatchTarget,
	frontends map[string]frontend.Frontend, done <-chan struct{},
) bool {
	token := project.ResolveGitHubTokenForTeam(target.ProjectAlias, target.Team)
	provider, err := gitprovider.NewProviderByNameWithToken(target.Provider, token)
	if err != nil {
		log.Printf("[prwatch] failed to create provider for %s: %v", target.Provider, err)
		return false
	}

	interval := prPollInitial
	deadline := time.NewTimer(prWatchTimeout)
	defer deadline.Stop()

	poll := time.NewTimer(interval)
	defer poll.Stop()

	lastDeliveredSHA := ""
	conflictNotified := false

	for {
		select {
		case <-done:
			return false
		case <-deadline.C:
			log.Printf("[prwatch] timeout for PR #%d %s/%s — stopping",
				target.PRIndex, target.Owner, target.Repo)
			return false
		case <-poll.C:
		}

		// Worker session gone → stop polling
		if !tmux.SessionExists(target.SessionName) {
			log.Printf("[prwatch] session %s gone — stopping PR #%d poll",
				target.SessionName, target.PRIndex)
			return false
		}

		// Check PR state
		fetchedPR, err := provider.GetPR(target.Owner, target.Repo, target.PRIndex)
		if err != nil {
			log.Printf("[prwatch] GetPR error for #%d: %v", target.PRIndex, err)
			interval = backoff(interval)
			poll.Reset(interval)
			continue
		}

		if fetchedPR.Merged || fetchedPR.State == "closed" {
			log.Printf("[prwatch] PR #%d is %s — stopping", target.PRIndex, fetchedPR.State)
			// Notifications now handled by on-modify hook → daemon RPC (taskComplete).
			// Return true to keep UUID in active map until the async cleanup
			// (task done) removes the task from ListTasksWithPR, preventing
			// a new goroutine from re-detecting the merge and double-notifying.
			return true
		}

		conflictNotified = checkMergeConflict(fetchedPR, target, frontends, conflictNotified)

		headSHA := fetchedPR.HeadSHA
		if headSHA == "" || headSHA == lastDeliveredSHA {
			interval = backoff(interval)
			poll.Reset(interval)
			continue
		}

		newInterval := handleCIStatus(provider, target, frontends, headSHA, interval)
		if newInterval == prPollInitial {
			lastDeliveredSHA = headSHA
		}
		interval = newInterval
		poll.Reset(interval)
	}
}

// handleCIStatus checks CI for a given SHA and delivers results.
// Returns prPollInitial to reset the backoff (CI resolved), or a backed-off interval to keep waiting.
// Callers use prPollInitial as a sentinel to update lastDeliveredSHA.
func handleCIStatus(
	provider gitprovider.Provider, target prWatchTarget,
	frontends map[string]frontend.Frontend, headSHA string, interval time.Duration,
) time.Duration {
	cs, err := provider.GetCombinedStatus(target.Owner, target.Repo, headSHA)
	if err != nil {
		log.Printf("[prwatch] GetCombinedStatus error for %s: %v", shortSHA(headSHA), err)
		return backoff(interval)
	}

	switch cs.State {
	case gitprovider.StateSuccess:
		msg := notification.CIPassed{
			Ctx:     notification.NewContext(target.ProjectAlias, target.TaskUUID, target.Description, ""),
			PRIndex: target.PRIndex,
			SHA:     headSHA,
		}.Render()
		deliverToWorkerSession(target, msg)
		log.Printf("[prwatch] PR #%d CI passed (sha=%s)", target.PRIndex, shortSHA(headSHA))
		// Return prPollInitial so the caller updates lastDeliveredSHA, preventing
		// re-notification for the same SHA. Goroutine stays alive to detect future
		// pushes and the eventual PR merge.
		return prPollInitial

	case gitprovider.StateFailure, gitprovider.StateError:
		ciFailedMsg := notification.CIFailed{
			Ctx:     notification.NewContext(target.ProjectAlias, target.TaskUUID, target.Description, ""),
			PRIndex: target.PRIndex,
			SHA:     headSHA,
		}.Render()
		deliverToWorkerSession(target, ciFailedMsg)
		notifyPRStatus(frontends, target, ciFailedMsg)
		log.Printf("[prwatch] PR #%d checks failed (sha=%s)", target.PRIndex, shortSHA(headSHA))
		return prPollInitial

	default:
		return backoff(interval)
	}
}

// checkMergeConflict notifies the worker once per conflict episode.
// Returns the updated conflictNotified flag.
func checkMergeConflict(
	pr *gitprovider.PullRequest, target prWatchTarget,
	frontends map[string]frontend.Frontend, alreadyNotified bool,
) bool {
	if pr.Mergeable {
		return false
	}
	if alreadyNotified {
		return true
	}
	conflictMsg := notification.MergeConflict{
		Ctx:     notification.NewContext(target.ProjectAlias, target.TaskUUID, target.Description, ""),
		PRIndex: target.PRIndex,
	}.Render()
	deliverToWorkerSession(target, conflictMsg)
	notifyPRStatus(frontends, target, conflictMsg)
	log.Printf("[prwatch] PR #%d has merge conflicts (sha=%s)", target.PRIndex, shortSHA(pr.HeadSHA))
	return true
}

// resolveWorkerWindowName returns the worker agent name (= tmux window name) for the given
// task tags by reading pipelines.toml. Falls back to worker.CoderAgentName if unavailable.
func resolveWorkerWindowName(taskTags []string) string {
	cfg, err := pipeline.Load(config.DefaultConfigDir())
	if err != nil {
		return worker.CoderAgentName
	}
	if name := cfg.WorkerAgentName(taskTags); name != "" {
		return name
	}
	if name := cfg.AnyWorkerAgentName(); name != "" {
		return name
	}
	return worker.CoderAgentName
}

// deliverToWorkerSession sends a message to the worker window of a worker tmux session.
func deliverToWorkerSession(target prWatchTarget, msg string) {
	if err := tmux.SendKeys(target.SessionName, target.WindowName, msg); err != nil {
		log.Printf("[prwatch] SendKeys failed for %s:%s: %v", target.SessionName, target.WindowName, err)
	}
}

// notifyPRStatus sends a pre-rendered notification message to the team's frontend.
func notifyPRStatus(
	frontends map[string]frontend.Frontend,
	target prWatchTarget, msg string,
) {
	team := target.Team
	if team == "" {
		team = config.DefaultTeamName
	}

	fe, ok := frontends[team]
	if !ok {
		log.Printf("[prwatch] notifyPRStatus: no frontend for team %q — notification dropped", team)
		return
	}

	if err := fe.SendNotification(context.Background(), msg); err != nil {
		log.Printf("[prwatch] notify failed: %v", err)
	}
}

// formatTaskDoneMsg returns the standard task-done message used for agent notifications.
func formatTaskDoneMsg(target prWatchTarget) string {
	return notification.TaskDone{
		Ctx:     notification.NewContext(target.ProjectAlias, target.TaskUUID, target.Description, ""),
		PRIndex: target.PRIndex,
	}.Render()
}

// notifySpawnerMerged delivers a PR-merged message to the spawning agent.
func notifySpawnerMerged(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend, target prWatchTarget,
) {
	if target.Spawner == "" {
		return
	}
	parts := strings.SplitN(target.Spawner, ":", 2)
	if len(parts) != 2 {
		log.Printf("[prwatch] notifySpawnerMerged: malformed spawner %q (want team:agent) — notification dropped",
			target.Spawner)
		return
	}
	teamName, agentName := parts[0], parts[1]
	if err := deliverToAgent(registry, mcfg, frontends, teamName, agentName, formatTaskDoneMsg(target)); err != nil {
		log.Printf("[prwatch] failed to notify spawner %s: %v", target.Spawner, err)
	}
}

// notifyManagerAgents delivers a task-done notification to manager agents in the task's
// owning team only. Skips any agent that is the same as the spawner (already notified).
func notifyManagerAgents(
	mcfg *config.DaemonConfig, registry *adapterRegistry,
	frontends map[string]frontend.Frontend, target prWatchTarget,
) {
	teamName := target.Team
	if teamName == "" {
		log.Printf("[prwatch] notifyManagerAgents: target.Team empty, falling back to default team")
		teamName = config.DefaultTeamName
	}
	team, ok := mcfg.Teams[teamName]
	if !ok {
		log.Printf("[prwatch] notifyManagerAgents: team %q not found in daemon config — notification dropped", teamName)
		return
	}
	if team.TeamPath == "" {
		log.Printf("[prwatch] notifyManagerAgents: team %q has no TeamPath configured — notification dropped", teamName)
		return
	}

	managers, err := agentfs.FindByRole(team.TeamPath, "manager")
	if err != nil {
		log.Printf("[prwatch] notifyManagerAgents: FindByRole for team %s: %v", teamName, err)
		return
	}
	msg := formatTaskDoneMsg(target)
	for _, agent := range managers {
		// Skip if this agent is the same as the spawner (already notified by notifySpawnerMerged)
		spawnerKey := teamName + ":" + agent.Name
		if spawnerKey == target.Spawner {
			continue
		}
		if err := deliverToAgent(registry, mcfg, frontends, teamName, agent.Name, msg); err != nil {
			log.Printf("[prwatch] notifyManagerAgents: deliver to %s/%s: %v", teamName, agent.Name, err)
		}
	}
}

func shortSHA(s string) string {
	if len(s) > 8 {
		return s[:8]
	}
	return s
}

func backoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * prPollBackoff)
	if next > prPollMax {
		return prPollMax
	}
	return next
}

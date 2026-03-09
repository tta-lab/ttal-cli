package daemon

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/gitprovider"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
	"github.com/tta-lab/ttal-cli/internal/tmux"
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
	TaskUUID    string
	SessionName string
	Team        string
	Owner       string
	Repo        string
	PRIndex     int64
	Description string
	Provider    string
	Spawner     string
}

// startPRWatcher periodically scans taskwarrior for pending tasks with pr_id set
// and active tmux sessions, then spawns per-PR polling goroutines.
func startPRWatcher(mcfg *config.DaemonConfig, registry *adapterRegistry, done <-chan struct{}) {
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

			scanForPRTasks(mcfg, registry, &mu, active, done)
			timer.Reset(prScanInterval)
		}
	}()

	log.Printf("[prwatch] scanner started (interval=%s)", prScanInterval)
}

// scanForPRTasks queries taskwarrior for tasks with pr_id, checks tmux sessions,
// and spawns polling goroutines for new PRs.
func scanForPRTasks(
	mcfg *config.DaemonConfig,
	registry *adapterRegistry,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) {
	seenUUIDs := make(map[string]bool)
	allSucceeded := true

	for teamName := range mcfg.Teams {
		seen := scanTeamWithEnv(mcfg, registry, teamName, mu, active, done)
		if seen == nil {
			// Team scan failed — skip pruning this round to avoid orphaning
			// goroutines whose UUIDs would be absent from an error-empty result.
			allSucceeded = false
			continue
		}
		for uuid := range seen {
			seenUUIDs[uuid] = true
		}
	}

	if !allSucceeded {
		return
	}

	// Prune UUIDs from active that no longer appear in any team's task list.
	// These are tasks that were merged/closed and have since been marked done.
	mu.Lock()
	for uuid := range active {
		if !seenUUIDs[uuid] {
			delete(active, uuid)
		}
	}
	mu.Unlock()
}

// scanTeamWithEnv sets TTAL_TEAM for non-default teams and delegates to scanTeam.
// Returns nil on taskwarrior error so the caller can skip the pruning pass.
func scanTeamWithEnv(
	mcfg *config.DaemonConfig,
	registry *adapterRegistry,
	teamName string,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) map[string]bool {
	if teamName == config.DefaultTeamName {
		return scanTeam(mcfg, registry, teamName, mu, active, done)
	}
	prev := os.Getenv("TTAL_TEAM")
	_ = os.Setenv("TTAL_TEAM", teamName)
	defer func() { _ = os.Setenv("TTAL_TEAM", prev) }()
	return scanTeam(mcfg, registry, teamName, mu, active, done)
}

func scanTeam(
	mcfg *config.DaemonConfig,
	registry *adapterRegistry,
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
		if task.ProjectPath == "" {
			continue
		}
		info, err := gitprovider.DetectProvider(task.ProjectPath)
		if err != nil {
			log.Printf("[prwatch] cannot detect provider for task %s: %v",
				shortSHA(task.UUID), err)
			continue
		}

		target := prWatchTarget{
			TaskUUID:    task.UUID,
			SessionName: sessionName,
			Team:        teamName,
			Owner:       info.Owner,
			Repo:        info.Repo,
			PRIndex:     prIndex,
			Description: task.Description,
			Provider:    string(info.Provider),
			Spawner:     task.Spawner,
		}

		mu.Lock()
		active[task.UUID] = true
		mu.Unlock()

		log.Printf("[prwatch] starting poll: PR #%d %s/%s session=%s",
			prIndex, info.Owner, info.Repo, sessionName)

		go func() {
			keep := pollPR(target, mcfg, registry, done)
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
func pollPR(target prWatchTarget, mcfg *config.DaemonConfig, registry *adapterRegistry, done <-chan struct{}) bool {
	provider, err := gitprovider.NewProviderByName(target.Provider)
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
			if fetchedPR.Merged {
				notifySpawnerMerged(mcfg, registry, target)
				notifyManagerAgents(mcfg, registry, target)
			}
			// Return true to keep UUID in active map until the async cleanup
			// (task done) removes the task from ListTasksWithPR, preventing
			// a new goroutine from re-detecting the merge and double-notifying.
			return true
		}

		conflictNotified = checkMergeConflict(fetchedPR, target, mcfg, conflictNotified)

		headSHA := fetchedPR.HeadSHA
		if headSHA == "" || headSHA == lastDeliveredSHA {
			interval = backoff(interval)
			poll.Reset(interval)
			continue
		}

		newInterval := handleCIStatus(provider, target, mcfg, headSHA, interval)
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
	mcfg *config.DaemonConfig, headSHA string, interval time.Duration,
) time.Duration {
	cs, err := provider.GetCombinedStatus(target.Owner, target.Repo, headSHA)
	if err != nil {
		log.Printf("[prwatch] GetCombinedStatus error for %s: %v", shortSHA(headSHA), err)
		return backoff(interval)
	}

	switch cs.State {
	case gitprovider.StateSuccess:
		log.Printf("[prwatch] PR #%d CI passed (sha=%s)", target.PRIndex, shortSHA(headSHA))
		deliverToWorkerSession(target.SessionName,
			fmt.Sprintf("✅ PR #%d CI checks passed (sha=%s)", target.PRIndex, shortSHA(headSHA)))
		// Return prPollInitial so the caller updates lastDeliveredSHA, preventing
		// re-notification for the same SHA. Goroutine stays alive to detect future
		// pushes and the eventual PR merge.
		return prPollInitial

	case gitprovider.StateFailure, gitprovider.StateError:
		msg, runURL := formatCIFailureWithURL(provider, target, headSHA)
		deliverToWorkerSession(target.SessionName, msg)
		notifyPRStatus(mcfg, target, "❌ CI failed", runURL)
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
	mcfg *config.DaemonConfig, alreadyNotified bool,
) bool {
	if pr.Mergeable {
		return false
	}
	if alreadyNotified {
		return true
	}
	msg := fmt.Sprintf("PR #%d has merge conflicts — rebase or merge base branch to resolve.",
		target.PRIndex)
	deliverToWorkerSession(target.SessionName, msg)
	notifyPRStatus(mcfg, target, "⚠️ Merge conflict detected", "")
	log.Printf("[prwatch] PR #%d has merge conflicts (sha=%s)", target.PRIndex, shortSHA(pr.HeadSHA))
	return true
}

// formatCIFailureWithURL builds a detailed failure message for the worker
// and returns the first run URL for Telegram notifications.
func formatCIFailureWithURL(provider gitprovider.Provider, target prWatchTarget, sha string) (string, string) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "PR #%d CI checks failed.\n", target.PRIndex)

	failures, err := provider.GetCIFailureDetails(target.Owner, target.Repo, sha)
	if err != nil {
		fmt.Fprintf(&sb, "Could not fetch failure details: %v\n", err)
		sb.WriteString("Fix the issues and push again.")
		return sb.String(), ""
	}

	if len(failures) == 0 {
		sb.WriteString("No detailed failure info available. Check CI directly.\n")
		sb.WriteString("Fix the issues and push again.")
		return sb.String(), ""
	}

	runURL := ""
	for _, f := range failures {
		fmt.Fprintf(&sb, "\nWorkflow: %s\n  Job: %s\n", f.WorkflowName, f.JobName)
		if f.HTMLURL != "" {
			fmt.Fprintf(&sb, "  URL: %s\n", f.HTMLURL)
			if runURL == "" {
				runURL = f.HTMLURL
			}
		}
		if f.LogTail != "" {
			sb.WriteString("  Log tail:\n")
			for _, line := range strings.Split(f.LogTail, "\n") {
				sb.WriteString("    " + line + "\n")
			}
		}
	}
	sb.WriteString("\nFix the issues and push again.")
	return sb.String(), runURL
}

// deliverToWorkerSession sends a message to the coder window of a worker tmux session.
func deliverToWorkerSession(sessionName, msg string) {
	coderWindow, err := tmux.FirstWindowExcept(sessionName, "review")
	if err != nil {
		log.Printf("[prwatch] failed to find coder window in %s: %v", sessionName, err)
		return
	}
	if coderWindow == "" {
		log.Printf("[prwatch] no coder window found in %s", sessionName)
		return
	}

	if err := tmux.SendKeys(sessionName, coderWindow, msg); err != nil {
		log.Printf("[prwatch] SendKeys failed for %s:%s: %v", sessionName, coderWindow, err)
	}
}

// notifyPRStatus sends PR status to the team's Telegram chat via the notification bot.
func notifyPRStatus(mcfg *config.DaemonConfig, target prWatchTarget, status string, runURL string) {
	team := target.Team
	if team == "" {
		team = config.DefaultTeamName
	}

	teamCfg, ok := mcfg.Teams[team]
	if !ok {
		log.Printf("[prwatch] notifyPRStatus: no config for team %q — notification dropped", team)
		return
	}

	msg := fmt.Sprintf("%s\nPR #%d: %s", status, target.PRIndex, target.Description)
	if runURL != "" {
		msg += "\n" + runURL
	}
	if err := notify.SendWithConfig(teamCfg.NotificationToken, teamCfg.ChatID, msg); err != nil {
		log.Printf("[prwatch] telegram notify failed: %v", err)
	}
}

// notifySpawnerMerged delivers a PR-merged message to the spawning agent.
func notifySpawnerMerged(mcfg *config.DaemonConfig, registry *adapterRegistry, target prWatchTarget) {
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
	msg := fmt.Sprintf("[task %s marked done, PR #%d merged] %s", target.TaskUUID[:8], target.PRIndex, target.Description)
	if err := deliverToAgent(registry, mcfg, teamName, agentName, msg); err != nil {
		log.Printf("[prwatch] failed to notify spawner %s: %v", target.Spawner, err)
	}
}

// notifyManagerAgents delivers a task-done notification to all agents with role "manager"
// across all teams. Skips any agent that is the same as the spawner (already notified).
func notifyManagerAgents(mcfg *config.DaemonConfig, registry *adapterRegistry, target prWatchTarget) {
	msg := fmt.Sprintf("[task %s done, PR #%d merged] %s", target.TaskUUID[:8], target.PRIndex, target.Description)

	for teamName, team := range mcfg.Teams {
		if team.TeamPath == "" {
			continue
		}
		managers, err := agentfs.FindByRole(team.TeamPath, "manager")
		if err != nil {
			log.Printf("[prwatch] notifyManagerAgents: failed to find managers in team %s: %v", teamName, err)
			continue
		}
		for _, agent := range managers {
			// Skip if this agent is the same as the spawner (already notified by notifySpawnerMerged)
			spawnerKey := teamName + ":" + agent.Name
			if spawnerKey == target.Spawner {
				continue
			}
			if err := deliverToAgent(registry, mcfg, teamName, agent.Name, msg); err != nil {
				log.Printf("[prwatch] failed to notify manager %s/%s: %v", teamName, agent.Name, err)
			}
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

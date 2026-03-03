package daemon

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

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
}

// startPRWatcher periodically scans taskwarrior for pending tasks with pr_id set
// and active tmux sessions, then spawns per-PR polling goroutines.
func startPRWatcher(mcfg *config.DaemonConfig, done <-chan struct{}) {
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

			scanForPRTasks(mcfg, &mu, active, done)
			timer.Reset(prScanInterval)
		}
	}()

	log.Printf("[prwatch] scanner started (interval=%s)", prScanInterval)
}

// scanForPRTasks queries taskwarrior for tasks with pr_id, checks tmux sessions,
// and spawns polling goroutines for new PRs.
func scanForPRTasks(
	mcfg *config.DaemonConfig,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) {
	// Iterate all teams to check their taskwarrior instances.
	for teamName := range mcfg.Teams {
		if teamName != config.DefaultTeamName {
			prev := os.Getenv("TTAL_TEAM")
			_ = os.Setenv("TTAL_TEAM", teamName)
			scanTeam(mcfg, teamName, mu, active, done)
			_ = os.Setenv("TTAL_TEAM", prev)
		} else {
			scanTeam(mcfg, teamName, mu, active, done)
		}
	}
}

func scanTeam(
	mcfg *config.DaemonConfig,
	teamName string,
	mu *sync.Mutex, active map[string]bool,
	done <-chan struct{},
) {
	tasks, err := taskwarrior.ListTasksWithPR()
	if err != nil {
		log.Printf("[prwatch] failed to list PR tasks for team %s: %v", teamName, err)
		return
	}

	for _, task := range tasks {
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

		prIndex, err := strconv.ParseInt(task.PRID, 10, 64)
		if err != nil {
			continue
		}

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
		}

		mu.Lock()
		active[task.UUID] = true
		mu.Unlock()

		log.Printf("[prwatch] starting poll: PR #%d %s/%s session=%s",
			prIndex, info.Owner, info.Repo, sessionName)

		go func() {
			pollPR(target, mcfg, done)
			mu.Lock()
			delete(active, target.TaskUUID)
			mu.Unlock()
		}()
	}
}

// pollPR polls a PR's CI status until checks resolve, PR is merged/closed, or timeout.
// Delivers status exactly once per HEAD SHA.
func pollPR(target prWatchTarget, mcfg *config.DaemonConfig, done <-chan struct{}) {
	provider, err := gitprovider.NewProviderByName(target.Provider)
	if err != nil {
		log.Printf("[prwatch] failed to create provider for %s: %v", target.Provider, err)
		return
	}

	interval := prPollInitial
	deadline := time.NewTimer(prWatchTimeout)
	defer deadline.Stop()

	poll := time.NewTimer(interval)
	defer poll.Stop()

	lastDeliveredSHA := ""

	for {
		select {
		case <-done:
			return
		case <-deadline.C:
			log.Printf("[prwatch] timeout for PR #%d %s/%s — stopping",
				target.PRIndex, target.Owner, target.Repo)
			return
		case <-poll.C:
		}

		// Worker session gone → stop polling
		if !tmux.SessionExists(target.SessionName) {
			log.Printf("[prwatch] session %s gone — stopping PR #%d poll",
				target.SessionName, target.PRIndex)
			return
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
			return
		}

		headSHA := fetchedPR.HeadSHA
		if headSHA == "" || headSHA == lastDeliveredSHA {
			interval = backoff(interval)
			poll.Reset(interval)
			continue
		}

		// Check CI status
		cs, err := provider.GetCombinedStatus(target.Owner, target.Repo, headSHA)
		if err != nil {
			log.Printf("[prwatch] GetCombinedStatus error for %s: %v", shortSHA(headSHA), err)
			interval = backoff(interval)
			poll.Reset(interval)
			continue
		}

		switch cs.State {
		case "success":
			msg := fmt.Sprintf("PR #%d checks passed — ready to merge. Run: ttal pr merge",
				target.PRIndex)
			deliverToWorkerSession(target.SessionName, msg)
			notifyPRStatus(mcfg, target, "✅ CI passed — ready to merge", "")
			log.Printf("[prwatch] PR #%d checks passed (sha=%s)", target.PRIndex, shortSHA(headSHA))
			return

		case "failure", "error":
			lastDeliveredSHA = headSHA
			msg, runURL := formatCIFailureWithURL(provider, target, headSHA)
			deliverToWorkerSession(target.SessionName, msg)
			notifyPRStatus(mcfg, target, "❌ CI failed", runURL)
			log.Printf("[prwatch] PR #%d checks failed (sha=%s)", target.PRIndex, shortSHA(headSHA))
			// Keep watching for new pushes
			interval = prPollInitial

		default:
			// "pending" or unknown — keep polling
			interval = backoff(interval)
		}

		poll.Reset(interval)
	}
}

// formatCIFailureWithURL builds a detailed failure message for the worker
// and returns the first run URL for Telegram notifications.
func formatCIFailureWithURL(provider gitprovider.Provider, target prWatchTarget, sha string) (string, string) {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("PR #%d CI checks failed.\n", target.PRIndex))

	failures, err := provider.GetCIFailureDetails(target.Owner, target.Repo, sha)
	if err != nil {
		sb.WriteString(fmt.Sprintf("Could not fetch failure details: %v\n", err))
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
		sb.WriteString(fmt.Sprintf("\nWorkflow: %s\n  Job: %s\n", f.WorkflowName, f.JobName))
		if f.HTMLURL != "" {
			sb.WriteString(fmt.Sprintf("  URL: %s\n", f.HTMLURL))
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

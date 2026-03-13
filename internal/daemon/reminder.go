package daemon

import (
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/notify"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const reminderPollInterval = 2 * time.Minute

// startReminderPoller polls taskwarrior for due reminders and sends Telegram notifications.
func startReminderPoller(mcfg *config.DaemonConfig, done <-chan struct{}) {
	// Validate team config once at startup — misconfiguration should be loud, not a recurring log.
	defaultTeam := mcfg.DefaultTeamName()
	team, ok := mcfg.Teams[defaultTeam]
	if !ok {
		log.Printf("[reminder] WARNING: default team %q not found in config — reminder poller disabled", defaultTeam)
		return
	}

	go func() {
		// Check immediately on startup (catch reminders that came due while daemon was down).
		fireReminders(team)

		ticker := time.NewTicker(reminderPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fireReminders(team)
			}
		}
	}()
}

func fireReminders(team *config.ResolvedTeam) {
	tasks, err := taskwarrior.GetDueReminders()
	if err != nil {
		log.Printf("[reminder] poll error: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	for _, t := range tasks {
		msg := "🔔 " + t.Description
		if err := notify.SendWithConfig(team.NotificationToken, team.ChatID, msg); err != nil {
			log.Printf("[reminder] failed to send for %s: %v", t.SessionID(), err)
			continue
		}
		if err := taskwarrior.MarkDone(t.UUID); err != nil {
			// Task stays pending and will be retried on the next poll cycle.
			log.Printf("[reminder] failed to mark done %s: %v", t.SessionID(), err)
			continue
		}
		log.Printf("[reminder] fired: %s", t.Description)
	}
}

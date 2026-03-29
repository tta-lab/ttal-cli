package daemon

import (
	"context"
	"log"
	"time"

	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/notification"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

const reminderPollInterval = 2 * time.Minute

// startReminderPoller polls taskwarrior for due reminders and sends frontend notifications.
func startReminderPoller(mcfg *config.DaemonConfig, frontends map[string]frontend.Frontend, done <-chan struct{}) {
	// Validate default team frontend once at startup — misconfiguration should be loud, not a recurring log.
	defaultTeam := mcfg.DefaultTeamName()
	fe, ok := frontends[defaultTeam]
	if !ok {
		log.Printf("[reminder] WARNING: no frontend for default team %q — reminder poller disabled", defaultTeam)
		return
	}

	go func() {
		// Check immediately on startup (catch reminders that came due while daemon was down).
		fireReminders(fe)

		ticker := time.NewTicker(reminderPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fireReminders(fe)
			}
		}
	}()
}

func fireReminders(fe frontend.Frontend) {
	tasks, err := taskwarrior.GetDueReminders()
	if err != nil {
		log.Printf("[reminder] poll error: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	for _, t := range tasks {
		msg := notification.Reminder{Ctx: notification.NewContext("", "", t.Description, "")}.Render()
		if err := fe.SendNotification(context.Background(), msg); err != nil {
			log.Printf("[reminder] failed to send for %s: %v", t.HexID(), err)
			continue
		}
		if err := taskwarrior.MarkDone(t.UUID); err != nil {
			// Task stays pending and will be retried on the next poll cycle.
			log.Printf("[reminder] failed to mark done %s: %v", t.HexID(), err)
			continue
		}
		log.Printf("[reminder] fired: %s", t.Description)
	}
}

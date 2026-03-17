package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/flicktask"
)

var remindCmd = &cobra.Command{
	Use:   "remind",
	Short: "Manage scheduled reminders (fires via Telegram when due)",
}

// --- remind add ---

var (
	remindAt string
	remindIn string
)

var remindAddCmd = &cobra.Command{
	Use:   "add [message]",
	Short: "Create a new reminder",
	Long: `Creates a flicktask task with +reminder tag and scheduled date.
The daemon polls for due reminders and sends Telegram notifications.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRemindAdd,
}

func runRemindAdd(cmd *cobra.Command, args []string) error {
	if remindAt == "" && remindIn == "" {
		return fmt.Errorf("specify when to remind\n\n  Example: ttal remind add \"check build\" --in 30m\n  Or:      ttal remind add \"standup\" --at 2026-03-18T09:00:00Z") //nolint:lll
	}
	if remindAt != "" && remindIn != "" {
		return fmt.Errorf("use --at or --in, not both\n\n  Example: ttal remind add \"check build\" --in 30m")
	}

	message := strings.Join(args, " ")

	var scheduled string
	if remindIn != "" {
		// flicktask --scheduled doesn't support now+30m relative offsets.
		// Parse the duration client-side and convert to absolute datetime.
		dur, err := time.ParseDuration(remindIn)
		if err != nil {
			return fmt.Errorf("invalid --in duration %q: %w\n\n  Use Go duration syntax: 30m, 2h, 1h30m", remindIn, err)
		}
		scheduled = time.Now().Add(dur).UTC().Format("2006-01-02T15:04:05Z")
	} else {
		scheduled = remindAt
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	teamName := cfg.TeamName()
	project := teamName + ".reminders"
	if agent := os.Getenv("TTAL_AGENT_NAME"); agent != "" {
		project = teamName + "." + agent
	}

	id, err := flicktask.AddTask(message,
		flicktask.WithProject(project),
		flicktask.WithTag("reminder"),
		flicktask.WithScheduled(scheduled),
	)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	fmt.Printf("⏰ Reminder set: %s\n", message)
	fmt.Printf("   ID: %s\n", id)
	fmt.Printf("   Scheduled: %s\n", scheduled)
	return nil
}

// --- remind list ---

var remindListCmd = &cobra.Command{
	Use:   "list",
	Short: "Show pending reminders",
	RunE:  runRemindList,
}

func runRemindList(_ *cobra.Command, _ []string) error {
	tasks, err := flicktask.GetPendingReminders()
	if err != nil {
		return fmt.Errorf("list reminders: %w", err)
	}
	if len(tasks) == 0 {
		fmt.Println("No pending reminders.")
		return nil
	}

	fmt.Printf("%-10s %-20s %s\n", "ID", "Scheduled", "Message")
	fmt.Printf("%-10s %-20s %s\n", "--", "---------", "-------")
	for _, t := range tasks {
		sched := t.Scheduled
		if sched != "" {
			for _, f := range []string{"20060102T150405Z", time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02T15:04:05"} {
				if parsed, err := time.Parse(f, sched); err == nil {
					sched = parsed.Local().Format("2006-01-02 15:04")
					break
				}
			}
		}
		fmt.Printf("%-10s %-20s %s\n", t.SessionID(), sched, t.Description)
	}
	return nil
}

// --- remind delete ---

var remindDeleteCmd = &cobra.Command{
	Use:   "delete [id]",
	Short: "Delete a reminder",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemindDelete,
}

func runRemindDelete(_ *cobra.Command, args []string) error {
	id := args[0]
	if err := flicktask.ValidateID(id); err != nil {
		return err
	}
	if err := flicktask.MarkDeleted(id); err != nil {
		return fmt.Errorf("delete reminder: %w", err)
	}
	fmt.Printf("Deleted reminder %s\n", id)
	return nil
}

func init() {
	rootCmd.AddCommand(remindCmd)

	remindAddCmd.Flags().StringVar(&remindAt, "at", "", "Absolute datetime in ISO 8601 format (e.g. '2026-03-18T14:00:00Z')")
	remindAddCmd.Flags().StringVar(&remindIn, "in", "", "Relative duration (e.g. '2h', '30m', '1h30m')")
	remindCmd.AddCommand(remindAddCmd)

	remindCmd.AddCommand(remindListCmd)
	remindCmd.AddCommand(remindDeleteCmd)
}

package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
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
	Long: `Creates a taskwarrior task with +reminder tag and scheduled date.
The daemon polls for due reminders and sends Telegram notifications.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runRemindAdd,
}

func runRemindAdd(cmd *cobra.Command, args []string) error {
	if remindAt == "" && remindIn == "" {
		return fmt.Errorf("specify --at or --in")
	}
	if remindAt != "" && remindIn != "" {
		return fmt.Errorf("use --at or --in, not both")
	}

	message := strings.Join(args, " ")

	// Both paths delegate to taskwarrior's native date parser.
	var scheduled string
	if remindIn != "" {
		scheduled = "now+" + remindIn
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

	uuid, err := taskwarrior.AddTask(message,
		"project:"+project,
		"+reminder",
		"scheduled:"+scheduled,
	)
	if err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}

	fmt.Printf("⏰ Reminder set: %s\n", message)
	fmt.Printf("   UUID: %s\n", uuid[:8])
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
	tasks, err := taskwarrior.GetPendingReminders()
	if err != nil {
		return err
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
			if parsed, err := time.Parse("20060102T150405Z", sched); err == nil {
				sched = parsed.Local().Format("2006-01-02 15:04")
			}
		}
		fmt.Printf("%-10s %-20s %s\n", t.SessionID(), sched, t.Description)
	}
	return nil
}

// --- remind delete ---

var remindDeleteCmd = &cobra.Command{
	Use:   "delete [uuid]",
	Short: "Delete a reminder",
	Args:  cobra.ExactArgs(1),
	RunE:  runRemindDelete,
}

func runRemindDelete(_ *cobra.Command, args []string) error {
	uuid := args[0]
	if err := taskwarrior.ValidateUUID(uuid); err != nil {
		return err
	}
	if err := taskwarrior.MarkDeleted(uuid); err != nil {
		return err
	}
	fmt.Printf("Deleted reminder %s\n", uuid)
	return nil
}

func init() {
	rootCmd.AddCommand(remindCmd)

	remindAddCmd.Flags().StringVar(&remindAt, "at", "", "Absolute time (e.g. '14:00', 'tomorrow', '2026-03-14T14:00')")
	remindAddCmd.Flags().StringVar(&remindIn, "in", "", "Relative duration (e.g. '2h', '30min', '1d')")
	remindCmd.AddCommand(remindAddCmd)

	remindCmd.AddCommand(remindListCmd)
	remindCmd.AddCommand(remindDeleteCmd)
}

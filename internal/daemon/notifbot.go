package daemon

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/telegram"
)

// notifBotCommands is the subset of commands registered on notification bots.
var notifBotCommands = []BotCommand{
	{Command: "status", Description: "Show all agents' context usage and stats"},
	{Command: "usage", Description: "Show Claude API 5hr/weekly rate limit consumption"},
	{Command: "restart", Description: "Restart the daemon (launchctl kickstart -k)"},
	{Command: "help", Description: "List available commands"},
}

// startNotificationPollers starts a poller for each team's notification bot token.
// The notification bot handles /status (all agents) and /restart (kickstart -k).
// Skips tokens that are already used as agent bot tokens (already polled).
func startNotificationPollers(mcfg *config.DaemonConfig, done chan struct{}) {
	agentTokens := make(map[string]bool)
	for _, ta := range mcfg.AllAgents() {
		if ta.Config.BotToken != "" {
			agentTokens[ta.Config.BotToken] = true
		}
	}

	for teamName, team := range mcfg.Teams {
		if team.NotificationToken == "" {
			continue
		}
		if agentTokens[team.NotificationToken] {
			log.Printf("[notifbot] skipping %s — token shared with agent bot (already polled)", teamName)
			continue
		}
		if team.ChatID == "" {
			log.Printf("[notifbot] skipping %s — no chat_id configured", teamName)
			continue
		}
		chatID, err := telegram.ParseChatID(team.ChatID)
		if err != nil {
			log.Printf("[notifbot] skipping %s — invalid chat_id: %v", teamName, err)
			continue
		}
		log.Printf("[notifbot] starting notification bot poller for team %s", teamName)
		startNotifBotPoller(team.NotificationToken, teamName, chatID, done)
	}
}

// startNotifBotPoller starts a long-poll loop for a notification bot token.
func startNotifBotPoller(botToken, teamName string, chatID int64, done <-chan struct{}) {
	go func() {
		backoff := 2 * time.Second
		for {
			select {
			case <-done:
				return
			default:
			}
			if err := runNotifBotPoller(botToken, teamName, chatID, done); err != nil {
				log.Printf("[notifbot] poller failed for %s: %v — retrying in %s", teamName, err, backoff)
				select {
				case <-done:
					return
				case <-time.After(backoff):
				}
				if backoff < 5*time.Minute {
					backoff *= 2
				}
			} else {
				backoff = 2 * time.Second
			}
		}
	}()
}

func runNotifBotPoller(botToken, teamName string, chatID int64, done <-chan struct{}) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		<-done
		cancel()
	}()

	matchCommand := func(cmd string) bot.MatchFunc {
		return func(update *models.Update) bool {
			if update.Message == nil || update.Message.Chat.ID != chatID {
				return false
			}
			for _, e := range update.Message.Entities {
				if e.Type != models.MessageEntityTypeBotCommand || e.Offset != 0 {
					continue
				}
				raw := update.Message.Text[1:e.Length]
				name, _, _ := strings.Cut(raw, "@")
				if name == cmd {
					return true
				}
			}
			return false
		}
	}

	defaultHandler := func(_ context.Context, _ *bot.Bot, _ *models.Update) {
		// Notification bot ignores non-command messages.
	}

	b, err := bot.New(botToken, bot.WithDefaultHandler(defaultHandler))
	if err != nil {
		return fmt.Errorf("bot init: %w", err)
	}

	// /status — show all agents for this team
	b.RegisterHandlerMatchFunc(matchCommand("status"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			args := parseCommandArgs(update.Message.Text)
			handleStatusCommand(teamName, "", botToken, chatIDStr, args)
		})

	// /usage — show Claude API rate limit consumption
	b.RegisterHandlerMatchFunc(matchCommand("usage"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			handleUsageCommand(botToken, chatIDStr)
		})

	// /restart — launchctl kickstart -k
	b.RegisterHandlerMatchFunc(matchCommand("restart"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			handleNotifRestart(botToken, chatIDStr)
		})

	// /help
	b.RegisterHandlerMatchFunc(matchCommand("help"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			handleNotifHelp(botToken, chatIDStr)
		})

	b.Start(ctx)
	return nil
}

// handleNotifRestart sends an ack and triggers launchctl kickstart -k to force-restart the daemon.
func handleNotifRestart(botToken, chatID string) {
	if err := telegram.SendMessage(botToken, chatID, "🔄 Daemon restarting..."); err != nil {
		log.Printf("[notifbot] failed to send restart ack: %v", err)
	}
	if err := Restart(); err != nil {
		log.Printf("[notifbot] restart failed: %v", err)
	}
}

func handleNotifHelp(botToken, chatID string) {
	var sb strings.Builder
	sb.WriteString("Notification bot commands:\n")
	for _, cmd := range notifBotCommands {
		fmt.Fprintf(&sb, "/%s — %s\n", cmd.Command, cmd.Description)
	}
	replyTelegram(botToken, chatID, sb.String())
}

package frontend

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
var notifBotCommands = []Command{
	{Name: "status", Description: "Show all agents' context usage and stats"},
	{Name: "usage", Description: "Show Claude API 5hr/weekly rate limit consumption"},
	{Name: "restart", Description: "Restart the daemon (launchctl kickstart -k)"},
	{Name: "help", Description: "List available commands"},
}

// StartNotificationPoller starts a poller for this team's notification bot token.
// The notification bot handles /status, /usage, /restart, /help.
// Skips if the notification token is shared with an agent bot (already polled).
func (f *TelegramFrontend) StartNotificationPoller(ctx context.Context) error {
	team, ok := f.cfg.MCfg.Teams[f.cfg.TeamName]
	if !ok || team.NotificationToken == "" {
		return nil
	}

	// Skip if this token is already used as an agent bot token (already polled).
	for _, ta := range f.cfg.MCfg.AllAgents() {
		if ta.TeamName != f.cfg.TeamName {
			continue
		}
		if config.AgentBotToken(ta.AgentName) == team.NotificationToken {
			log.Printf("[notifbot] skipping %s — token shared with agent bot (already polled)", f.cfg.TeamName)
			return nil
		}
	}

	if team.ChatID == "" {
		log.Printf("[notifbot] skipping %s — no chat_id configured", f.cfg.TeamName)
		return nil
	}
	chatID, err := telegram.ParseChatID(team.ChatID)
	if err != nil {
		log.Printf("[notifbot] skipping %s — invalid chat_id: %v", f.cfg.TeamName, err)
		return nil
	}

	log.Printf("[notifbot] starting notification bot poller for team %s", f.cfg.TeamName)
	f.startNotifBotPoller(team.NotificationToken, chatID, ctx)
	return nil
}

// startNotifBotPoller starts a long-poll loop for a notification bot token.
func (f *TelegramFrontend) startNotifBotPoller(botToken string, chatID int64, ctx context.Context) {
	go func() {
		backoff := 2 * time.Second
		for {
			select {
			case <-f.done:
				return
			default:
			}
			if err := f.runNotifBotPoller(botToken, chatID, ctx); err != nil {
				log.Printf("[notifbot] poller failed for %s: %v — retrying in %s", f.cfg.TeamName, err, backoff)
				select {
				case <-f.done:
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

func (f *TelegramFrontend) runNotifBotPoller(botToken string, chatID int64, parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case <-f.done:
			cancel()
		case <-parentCtx.Done():
			cancel()
		}
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

	b.RegisterHandlerMatchFunc(matchCommand("status"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			args := parseCommandArgs(update.Message.Text)
			f.handleStatusCommand(f.cfg.TeamName, "", botToken, chatIDStr, args)
		})

	b.RegisterHandlerMatchFunc(matchCommand("usage"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			f.handleUsageCommand(botToken, chatIDStr)
		})

	b.RegisterHandlerMatchFunc(matchCommand("restart"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			if err := telegram.SendMessage(botToken, chatIDStr, "🔄 Daemon restarting..."); err != nil {
				log.Printf("[notifbot] failed to send restart ack: %v", err)
			}
			if f.cfg.RestartFn != nil {
				if err := f.cfg.RestartFn(); err != nil {
					log.Printf("[notifbot] restart failed: %v", err)
				}
			}
		})

	b.RegisterHandlerMatchFunc(matchCommand("help"),
		func(_ context.Context, _ *bot.Bot, update *models.Update) {
			chatIDStr := fmt.Sprintf("%d", update.Message.Chat.ID)
			var sb strings.Builder
			sb.WriteString("Notification bot commands:\n")
			for _, cmd := range notifBotCommands {
				fmt.Fprintf(&sb, "/%s — %s\n", cmd.Name, cmd.Description)
			}
			replyTelegram(botToken, chatIDStr, sb.String())
		})

	b.Start(ctx)
	return nil
}

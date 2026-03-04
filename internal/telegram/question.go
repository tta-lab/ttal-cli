package telegram

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// QuestionPage holds the data needed to render one page of a question batch.
type QuestionPage struct {
	AgentName      string
	PageNum        int // 1-indexed
	TotalPages     int
	Header         string
	Text           string
	Options        []QuestionPageOption
	AllowCustom    bool
	SelectedAnswer string // Non-empty if this question was already answered
	AllAnswered    bool   // True if all questions in batch are answered
	CallbackPrefix string // Short ID for callback data
	QuestionIdx    int    // 0-indexed question index
}

// QuestionPageOption is one option for display in the inline keyboard.
type QuestionPageOption struct {
	Label       string
	Description string
	Selected    bool
}

// RenderQuestionPage builds the message text and inline keyboard for one question page.
func RenderQuestionPage(p QuestionPage) (string, *models.InlineKeyboardMarkup) {
	var sb strings.Builder
	if p.TotalPages > 1 {
		fmt.Fprintf(&sb, "❓ <b>%s</b> asks [%d/%d — %s]:\n", p.AgentName, p.PageNum, p.TotalPages, p.Header)
	} else {
		fmt.Fprintf(&sb, "❓ <b>%s</b> asks [%s]:\n", p.AgentName, p.Header)
	}

	if p.SelectedAnswer != "" {
		fmt.Fprintf(&sb, "✅ Selected: %s\n", p.SelectedAnswer)
	}

	fmt.Fprintf(&sb, "\n%s", p.Text)

	// If any option has a description, render numbered list in body
	hasDescriptions := false
	for _, opt := range p.Options {
		if opt.Description != "" {
			hasDescriptions = true
			break
		}
	}

	if hasDescriptions {
		sb.WriteString("\n")
		for i, opt := range p.Options {
			if opt.Description != "" {
				fmt.Fprintf(&sb, "\n%s <b>%s</b> — %s", numberEmoji(i+1), escapeHTML(opt.Label), escapeHTML(opt.Description))
			} else {
				fmt.Fprintf(&sb, "\n%s <b>%s</b>", numberEmoji(i+1), escapeHTML(opt.Label))
			}
		}
		sb.WriteString("\n")
	}

	text := sb.String()

	markup := buildQuestionKeyboard(p, hasDescriptions)
	return text, markup
}

func buildQuestionKeyboard(p QuestionPage, hasDescriptions bool) *models.InlineKeyboardMarkup {
	rows := make([][]models.InlineKeyboardButton, 0, len(p.Options)+2)
	for i, opt := range p.Options {
		label := opt.Label
		if hasDescriptions {
			label = fmt.Sprintf("%s %s", numberEmoji(i+1), opt.Label)
		}
		if opt.Selected {
			label = "✅ " + label
		}
		callbackData := fmt.Sprintf("q:%s:%d:%d", p.CallbackPrefix, p.QuestionIdx, i)
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: label, CallbackData: callbackData},
		})
	}

	if p.AllowCustom {
		rows = append(rows, []models.InlineKeyboardButton{
			{Text: "✏️ Type my own answer", CallbackData: fmt.Sprintf("q:%s:%d:custom", p.CallbackPrefix, p.QuestionIdx)},
		})
	}

	// Skip button — always present
	rows = append(rows, []models.InlineKeyboardButton{
		{Text: "❌ Skip", CallbackData: fmt.Sprintf("qskip:%s", p.CallbackPrefix)},
	})

	var navRow []models.InlineKeyboardButton
	if p.PageNum > 1 {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text: "← Prev", CallbackData: fmt.Sprintf("qnav:%s:prev", p.CallbackPrefix),
		})
	}
	if p.AllAnswered {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text: "✅ Submit", CallbackData: fmt.Sprintf("qsubmit:%s", p.CallbackPrefix),
		})
	}
	if p.PageNum < p.TotalPages {
		navRow = append(navRow, models.InlineKeyboardButton{
			Text: "Next →", CallbackData: fmt.Sprintf("qnav:%s:next", p.CallbackPrefix),
		})
	}
	if len(navRow) > 0 {
		rows = append(rows, navRow)
	}

	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

// RenderCustomInputPrompt renders the "waiting for typed answer" page.
func RenderCustomInputPrompt(p QuestionPage) (string, *models.InlineKeyboardMarkup) {
	var sb strings.Builder
	if p.TotalPages > 1 {
		fmt.Fprintf(&sb, "❓ <b>%s</b> asks [%d/%d — %s]:\n", p.AgentName, p.PageNum, p.TotalPages, p.Header)
	} else {
		fmt.Fprintf(&sb, "❓ <b>%s</b> asks [%s]:\n", p.AgentName, p.Header)
	}
	fmt.Fprintf(&sb, "\n%s\n\n⌨️ <i>Type your answer below...</i>", p.Text)

	cancelRow := []models.InlineKeyboardButton{
		{Text: "✖ Cancel — show options", CallbackData: fmt.Sprintf("qnav:%s:cancel_custom", p.CallbackPrefix)},
	}
	markup := &models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{cancelRow}}
	return sb.String(), markup
}

// RenderSubmittedSummary renders the final "answers submitted" summary.
func RenderSubmittedSummary(agentName string, questions []string, answers []string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "✅ <b>%s</b> — %d answer(s) submitted:\n", agentName, len(answers))
	for i, q := range questions {
		ans := ""
		if i < len(answers) {
			ans = answers[i]
		}
		fmt.Fprintf(&sb, "\n%d. %s → <b>%s</b>", i+1, truncate(q, 60), ans)
	}
	return sb.String()
}

// SendQuestionMessage sends a question with inline keyboard buttons.
func SendQuestionMessage(botToken string, chatID int64, text string, markup *models.InlineKeyboardMarkup) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := bot.New(botToken)
	if err != nil {
		return 0, fmt.Errorf("telegram bot init: %w", err)
	}

	msg, err := b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	if err != nil {
		return 0, fmt.Errorf("telegram send question: %w", err)
	}
	return msg.ID, nil
}

// EditQuestionMessage edits a question message in-place (for pagination/updates).
func EditQuestionMessage(
	botToken string, chatID int64, messageID int,
	text string, markup *models.InlineKeyboardMarkup,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	b, err := bot.New(botToken)
	if err != nil {
		return fmt.Errorf("telegram bot init: %w", err)
	}

	_, err = b.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      chatID,
		MessageID:   messageID,
		Text:        text,
		ParseMode:   models.ParseModeHTML,
		ReplyMarkup: markup,
	})
	return err
}

// numberEmoji returns a number emoji for 1-9, falls back to "N." for 10+.
func numberEmoji(n int) string {
	emojis := []string{"1️⃣", "2️⃣", "3️⃣", "4️⃣", "5️⃣", "6️⃣", "7️⃣", "8️⃣", "9️⃣"}
	if n >= 1 && n <= len(emojis) {
		return emojis[n-1]
	}
	return fmt.Sprintf("%d.", n)
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

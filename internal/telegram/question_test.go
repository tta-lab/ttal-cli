package telegram

import (
	"strings"
	"testing"
)

func TestRenderQuestionPage_SingleQuestion(t *testing.T) {
	page := QuestionPage{
		AgentName:      "kestrel",
		PageNum:        1,
		TotalPages:     1,
		Header:         "Database",
		Text:           "Which database should we use?",
		Options:        []QuestionPageOption{{Label: "PostgreSQL"}, {Label: "MongoDB"}},
		AllowCustom:    true,
		CallbackPrefix: "000001",
		QuestionIdx:    0,
	}

	text, markup := RenderQuestionPage(page)

	if !strings.Contains(text, "kestrel") {
		t.Error("expected agent name in text")
	}
	if !strings.Contains(text, "Database") {
		t.Error("expected header in text")
	}
	if !strings.Contains(text, "Which database") {
		t.Error("expected question text")
	}
	// No page indicator for single question
	if strings.Contains(text, "1/1") {
		t.Error("should not show page indicator for single question")
	}

	// 2 options + 1 custom = 3 rows
	if len(markup.InlineKeyboard) != 3 {
		t.Errorf("expected 3 keyboard rows, got %d", len(markup.InlineKeyboard))
	}

	// Verify callback data format
	cb := markup.InlineKeyboard[0][0].CallbackData
	if cb != "q:000001:0:0" {
		t.Errorf("callback data = %q, want %q", cb, "q:000001:0:0")
	}

	customCB := markup.InlineKeyboard[2][0].CallbackData
	if customCB != "q:000001:0:custom" {
		t.Errorf("custom callback = %q, want %q", customCB, "q:000001:0:custom")
	}
}

func TestRenderQuestionPage_MultiQuestion(t *testing.T) {
	page := QuestionPage{
		AgentName:      "yuki",
		PageNum:        1,
		TotalPages:     3,
		Header:         "Auth",
		Text:           "Which auth method?",
		Options:        []QuestionPageOption{{Label: "JWT"}, {Label: "OAuth"}},
		AllowCustom:    false,
		AllAnswered:    false,
		CallbackPrefix: "000002",
		QuestionIdx:    0,
	}

	text, markup := RenderQuestionPage(page)

	if !strings.Contains(text, "1/3") {
		t.Error("expected page indicator for multi-question")
	}

	// 2 options + 1 nav row (Next only, page 1)
	if len(markup.InlineKeyboard) != 3 {
		t.Errorf("expected 3 rows, got %d", len(markup.InlineKeyboard))
	}

	// Nav row should have Next button only
	navRow := markup.InlineKeyboard[2]
	if len(navRow) != 1 || navRow[0].Text != "Next →" {
		t.Errorf("expected single Next button, got %v", navRow)
	}
}

func TestRenderQuestionPage_SubmitButton(t *testing.T) {
	page := QuestionPage{
		AgentName:      "yuki",
		PageNum:        2,
		TotalPages:     2,
		Header:         "Auth",
		Text:           "Q2",
		Options:        []QuestionPageOption{{Label: "A"}},
		AllAnswered:    true,
		CallbackPrefix: "000003",
		QuestionIdx:    1,
	}

	_, markup := RenderQuestionPage(page)

	// Find Submit button
	found := false
	for _, row := range markup.InlineKeyboard {
		for _, btn := range row {
			if strings.Contains(btn.Text, "Submit") {
				found = true
				if btn.CallbackData != "qsubmit:000003" {
					t.Errorf("submit callback = %q, want %q", btn.CallbackData, "qsubmit:000003")
				}
			}
		}
	}
	if !found {
		t.Error("expected Submit button when all answered")
	}
}

func TestRenderQuestionPage_SelectedOption(t *testing.T) {
	page := QuestionPage{
		AgentName:      "test",
		PageNum:        1,
		TotalPages:     1,
		Header:         "H",
		Text:           "Q",
		Options:        []QuestionPageOption{{Label: "A", Selected: true}, {Label: "B"}},
		SelectedAnswer: "A",
		CallbackPrefix: "000004",
	}

	text, markup := RenderQuestionPage(page)

	if !strings.Contains(text, "Selected: A") {
		t.Error("expected selected answer in text")
	}
	if !strings.HasPrefix(markup.InlineKeyboard[0][0].Text, "✅") {
		t.Error("expected checkmark on selected option")
	}
}

func TestRenderCustomInputPrompt(t *testing.T) {
	page := QuestionPage{
		AgentName:      "test",
		PageNum:        1,
		TotalPages:     1,
		Header:         "H",
		Text:           "Q",
		CallbackPrefix: "000005",
	}

	text, markup := RenderCustomInputPrompt(page)

	if !strings.Contains(text, "Type your answer") {
		t.Error("expected custom input prompt")
	}
	if len(markup.InlineKeyboard) != 1 {
		t.Fatalf("expected 1 row (cancel), got %d", len(markup.InlineKeyboard))
	}
	if !strings.Contains(markup.InlineKeyboard[0][0].CallbackData, "cancel_custom") {
		t.Error("expected cancel_custom callback")
	}
}

func TestRenderSubmittedSummary(t *testing.T) {
	summary := RenderSubmittedSummary("kestrel", []string{"Q1", "Q2"}, []string{"A1", "A2"})

	if !strings.Contains(summary, "kestrel") {
		t.Error("expected agent name")
	}
	if !strings.Contains(summary, "2 answer(s)") {
		t.Error("expected answer count")
	}
	if !strings.Contains(summary, "A1") || !strings.Contains(summary, "A2") {
		t.Error("expected answers in summary")
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is too long", 10, "this is..."},
		{"abc", 3, "abc"},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestRenderQuestionPage_OptionWithDescription(t *testing.T) {
	page := QuestionPage{
		AgentName:      "test",
		PageNum:        1,
		TotalPages:     1,
		Header:         "H",
		Text:           "Q",
		Options:        []QuestionPageOption{{Label: "A", Description: "desc A"}},
		CallbackPrefix: "000006",
	}

	_, markup := RenderQuestionPage(page)

	btnText := markup.InlineKeyboard[0][0].Text
	if !strings.Contains(btnText, "A — desc A") {
		t.Errorf("expected label with description, got %q", btnText)
	}
}

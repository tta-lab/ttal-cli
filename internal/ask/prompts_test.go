package ask

import (
	"strings"
	"testing"
)

func TestGeneralPromptHasPlaceholder(t *testing.T) {
	if !strings.Contains(generalPrompt, "{cwd}") {
		t.Error("general.md prompt is missing {cwd} placeholder — " +
			"BuildSystemPromptForMode will silently omit the working directory")
	}
}

func TestModeValid(t *testing.T) {
	valid := []Mode{ModeProject, ModeRepo, ModeURL, ModeWeb, ModeGeneral}
	for _, m := range valid {
		if !m.Valid() {
			t.Errorf("expected %q to be valid", m)
		}
	}

	invalid := []Mode{"", "unknown", "GENERAL", "Project"}
	for _, m := range invalid {
		if m.Valid() {
			t.Errorf("expected %q to be invalid", m)
		}
	}
}

package cmd

import (
	"testing"

	"github.com/tta-lab/ttal-cli/internal/sync"
)

func TestCountUniqueSkills(t *testing.T) {
	results := []sync.SkillResult{
		{Name: "a"},
		{Name: "a"},
		{Name: "b"},
	}

	got := countUniqueSkills(results)
	if got != 2 {
		t.Fatalf("countUniqueSkills() = %d, want %d", got, 2)
	}
}

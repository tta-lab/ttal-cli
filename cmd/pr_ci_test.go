package cmd

import (
	"testing"
)

func TestPRCICommandExists(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "ci"})
	if err != nil {
		t.Fatalf("pr ci command not found: %v", err)
	}
	if cmd.Name() != "ci" {
		t.Errorf("expected command name 'ci', got %q", cmd.Name())
	}
}

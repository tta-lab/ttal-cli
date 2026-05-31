package cmd

import "testing"

func TestPRViewCommandExists(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "view"})
	if err != nil {
		t.Fatalf("pr view command not found: %v", err)
	}
	if cmd.Name() != "view" {
		t.Errorf("expected command name 'view', got %q", cmd.Name())
	}
	if len(prViewCmd.Aliases) != 1 || prViewCmd.Aliases[0] != "list" {
		t.Errorf("expected alias 'list', got %v", prViewCmd.Aliases)
	}
}

func TestPRListAliasWorks(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"pr", "list"})
	if err != nil {
		t.Fatalf("pr list alias not found: %v", err)
	}
	if cmd.Name() != "view" {
		t.Errorf("expected alias 'list' to resolve to 'view', got %q", cmd.Name())
	}
}

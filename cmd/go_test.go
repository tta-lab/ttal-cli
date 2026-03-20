package cmd

import (
	"testing"
)

func TestGoCmdExists(t *testing.T) {
	// Verify go subcommand is registered on the task command.
	var found bool
	for _, sub := range taskCmd.Commands() {
		if sub.Name() == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal task go command not found")
	}
}

func TestGoCmd_RequiresUUID(t *testing.T) {
	// Go requires exactly one UUID argument.
	if taskGoCmd.Args == nil {
		t.Error("expected Args validator on go command")
	}
}

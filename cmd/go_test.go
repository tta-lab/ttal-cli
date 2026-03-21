package cmd

import (
	"testing"
)

func TestGoCmdExists(t *testing.T) {
	// Verify go is registered as a top-level command on rootCmd.
	var found bool
	for _, sub := range rootCmd.Commands() {
		if sub.Name() == "go" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal go command not found on root")
	}
}

func TestGoCmd_RequiresUUID(t *testing.T) {
	// Go requires exactly one UUID argument.
	if goCmd.Args == nil {
		t.Error("expected Args validator on go command")
	}
}

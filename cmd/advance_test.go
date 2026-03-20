package cmd

import (
	"testing"
)

func TestAdvanceCmdExists(t *testing.T) {
	// Verify advance subcommand is registered on the task command.
	var found bool
	for _, sub := range taskCmd.Commands() {
		if sub.Name() == "advance" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ttal task advance command not found")
	}
}

func TestAdvanceCmd_RequiresUUID(t *testing.T) {
	// Advance requires exactly one UUID argument.
	if taskAdvanceCmd.Args == nil {
		t.Error("expected Args validator on advance command")
	}
}

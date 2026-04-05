package cmd

import (
	"testing"
)

func TestLogFlagDefaults(t *testing.T) {
	fs := logCmd.Flags()

	tailVal, err := fs.GetInt("tail")
	if err != nil {
		t.Fatalf("GetInt(tail) error: %v", err)
	}
	if tailVal != 100 {
		t.Errorf("tail default = %d, want 100", tailVal)
	}

	sinceVal, err := fs.GetString("since")
	if err != nil {
		t.Fatalf("GetString(since) error: %v", err)
	}
	if sinceVal != "" {
		t.Errorf("since default = %q, want empty string", sinceVal)
	}
}

func TestLogFlagParsing(t *testing.T) {
	fs := logCmd.Flags()

	if err := fs.Parse([]string{"--tail", "500"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	tailVal, _ := fs.GetInt("tail")
	if tailVal != 500 {
		t.Errorf("tail = %d, want 500", tailVal)
	}

	if err := fs.Parse([]string{"--since", "5m"}); err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	sinceVal, _ := fs.GetString("since")
	if sinceVal != "5m" {
		t.Errorf("since = %q, want %q", sinceVal, "5m")
	}
}

func TestLogCmdUse(t *testing.T) {
	if logCmd.Use != "log <project-alias>" {
		t.Errorf("Use = %q, want %q", logCmd.Use, "log <project-alias>")
	}
	if logCmd.Args == nil {
		t.Error("Args should be set to ExactArgs(1)")
	}
}

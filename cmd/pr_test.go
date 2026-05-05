package cmd

import (
	"strings"
	"testing"
)

func TestPRModifyCmd_FlagRegistration(t *testing.T) {
	titleFlag := prModifyCmd.Flag("title")
	if titleFlag == nil {
		t.Fatal("expected --title flag on prModifyCmd")
	}
	prIDFlag := prModifyCmd.Flag("pr-id")
	if prIDFlag == nil {
		t.Fatal("expected --pr-id flag on prModifyCmd")
	}
	bodyFlag := prModifyCmd.Flag("body")
	if bodyFlag != nil {
		t.Error("--body flag should NOT exist on prModifyCmd")
	}
}

func TestPRCreateCmd_NoBodyFlag(t *testing.T) {
	bodyFlag := prCreateCmd.Flag("body")
	if bodyFlag != nil {
		t.Error("--body flag should NOT exist on prCreateCmd")
	}
	prIDFlag := prCreateCmd.Flag("pr-id")
	if prIDFlag != nil {
		t.Error("--pr-id flag should NOT exist on prCreateCmd")
	}
}

func TestPRModifyCmd_HelpReflectsNewContract(t *testing.T) {
	var buf strings.Builder
	prModifyCmd.SetOut(&buf)
	if err := prModifyCmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	helpText := buf.String()
	if !strings.Contains(helpText, "stdin") && !strings.Contains(helpText, "heredoc") {
		t.Error("help text should mention stdin or heredoc, got:\n" + helpText)
	}
	if strings.Contains(helpText, "--body") {
		t.Error("help text should NOT mention --body")
	}
}

func TestPRCreateCmd_HelpReflectsNewContract(t *testing.T) {
	var buf strings.Builder
	prCreateCmd.SetOut(&buf)
	if err := prCreateCmd.Help(); err != nil {
		t.Fatalf("help: %v", err)
	}
	helpText := buf.String()
	if !strings.Contains(helpText, "stdin") && !strings.Contains(helpText, "heredoc") {
		t.Error("help text should mention stdin or heredoc, got:\n" + helpText)
	}
	if strings.Contains(helpText, "--body") {
		t.Error("help text should NOT mention --body")
	}
}

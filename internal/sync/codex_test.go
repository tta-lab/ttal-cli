package sync

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateCodexVariant(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:        "test-agent",
			Description: "Test description",
			Codex: map[string]interface{}{
				"model":                  "gpt-5.2-codex",
				"model_reasoning_effort": "low",
			},
		},
		Body: "You are a test agent.\n\n## Rules\n- Be helpful\n",
	}

	result := GenerateCodexVariant(agent)

	if !strings.Contains(result, codexManagedMarker) {
		t.Error("should contain managed marker")
	}
	if !strings.Contains(result, `model = "gpt-5.2-codex"`) {
		t.Error("should contain model")
	}
	if !strings.Contains(result, `model_reasoning_effort = "low"`) {
		t.Error("should contain model_reasoning_effort")
	}
	if !strings.Contains(result, `developer_instructions = """`) {
		t.Error("should contain developer_instructions")
	}
	if !strings.Contains(result, "You are a test agent.") {
		t.Error("should contain body content")
	}
}

func TestGenerateCodexVariantNoCodexBlock(t *testing.T) {
	agent := &ParsedAgent{
		Frontmatter: AgentFrontmatter{
			Name:        "minimal",
			Description: "No codex block",
		},
		Body: "Prompt body.\n",
	}

	result := GenerateCodexVariant(agent)

	if !strings.Contains(result, codexManagedMarker) {
		t.Error("should contain managed marker")
	}
	if !strings.Contains(result, "Prompt body.") {
		t.Error("should contain body")
	}
	// Should not have model line
	if strings.Contains(result, "model =") {
		t.Error("should not have model when codex block absent")
	}
}

func TestDeployCodexAgents(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agents := []*ParsedAgent{
		{
			Frontmatter: AgentFrontmatter{
				Name:        "researcher",
				Description: "Research-focused agent.",
				Codex: map[string]interface{}{
					"model": "gpt-5.2-codex",
				},
			},
			Body: "You are a researcher.",
		},
		{
			Frontmatter: AgentFrontmatter{
				Name:        "task-creator",
				Description: "Mechanical task creation.",
			},
			Body: "You create tasks.",
		},
	}

	if err := DeployCodexAgents(agents, false); err != nil {
		t.Fatalf("DeployCodexAgents: %v", err)
	}

	// Verify per-agent .toml files
	researcherPath := filepath.Join(tmpHome, ".codex", "agents", "researcher.toml")
	content, err := os.ReadFile(researcherPath)
	if err != nil {
		t.Fatalf("reading researcher.toml: %v", err)
	}
	if !strings.Contains(string(content), codexManagedMarker) {
		t.Error("researcher.toml should contain managed marker")
	}
	if !strings.Contains(string(content), `model = "gpt-5.2-codex"`) {
		t.Error("researcher.toml should contain model")
	}

	taskCreatorPath := filepath.Join(tmpHome, ".codex", "agents", "task-creator.toml")
	if _, err := os.Stat(taskCreatorPath); err != nil {
		t.Fatalf("task-creator.toml should exist: %v", err)
	}

	// Verify config.toml
	configPath := filepath.Join(tmpHome, ".codex", "config.toml")
	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	configStr := string(configContent)

	if !strings.Contains(configStr, "multi_agent = true") {
		t.Error("config.toml should enable multi_agent")
	}
	if !strings.Contains(configStr, "[agents.researcher]") {
		t.Error("config.toml should contain researcher registration")
	}
	if !strings.Contains(configStr, "[agents.task-creator]") {
		t.Error("config.toml should contain task-creator registration")
	}
	if !strings.Contains(configStr, `"./agents/researcher.toml"`) {
		t.Error("config.toml should reference researcher config file")
	}
}

func TestDeployCodexAgentsDryRun(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	agents := []*ParsedAgent{
		{
			Frontmatter: AgentFrontmatter{
				Name:        "dry-agent",
				Description: "Should not be written.",
			},
			Body: "body",
		},
	}

	if err := DeployCodexAgents(agents, true); err != nil {
		t.Fatalf("DeployCodexAgents dry-run: %v", err)
	}

	// Nothing should be written
	if _, err := os.Stat(filepath.Join(tmpHome, ".codex", "agents", "dry-agent.toml")); !os.IsNotExist(err) {
		t.Error("dry-run should not create agent file")
	}
	if _, err := os.Stat(filepath.Join(tmpHome, ".codex", "config.toml")); !os.IsNotExist(err) {
		t.Error("dry-run should not create config.toml")
	}
}

func TestDeployAgentsWithCodex(t *testing.T) {
	srcDir := t.TempDir()
	agentContent := `---
name: test-bot
description: A test bot

claude-code:
  model: haiku

codex:
  model: gpt-5.2-codex
---

You are a test bot.
`
	if err := os.WriteFile(filepath.Join(srcDir, "test-bot.md"), []byte(agentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	results, err := DeployAgents([]string{srcDir}, false)
	if err != nil {
		t.Fatalf("DeployAgents: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	r := results[0]
	if r.CodexDest == "" {
		t.Error("CodexDest should be set")
	}

	// Verify Codex agent file was written
	codexContent, err := os.ReadFile(filepath.Join(tmpHome, ".codex", "agents", "test-bot.toml"))
	if err != nil {
		t.Fatalf("reading Codex output: %v", err)
	}
	if !strings.Contains(string(codexContent), `model = "gpt-5.2-codex"`) {
		t.Error("Codex file should contain model")
	}

	// Verify config.toml was written
	configContent, err := os.ReadFile(filepath.Join(tmpHome, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	if !strings.Contains(string(configContent), "[agents.test-bot]") {
		t.Error("config.toml should register test-bot")
	}
	if !strings.Contains(string(configContent), "multi_agent = true") {
		t.Error("config.toml should enable multi_agent")
	}
}

func TestMergePreservesExistingConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	codexDir := filepath.Join(tmpHome, ".codex")
	agentsDir := filepath.Join(codexDir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing config.toml with user content
	existingConfig := `model = "gpt-5.2-codex"

[features]
multi_agent = true

[agents]
max_threads = 5

[agents.user-agent]
description = "User's custom agent"
config_file = "./agents/user-agent.toml"
`
	configPath := filepath.Join(codexDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(existingConfig), 0o644); err != nil {
		t.Fatal(err)
	}

	// User's custom agent file (not managed by ttal)
	userAgentContent := "developer_instructions = \"Custom agent\"\n"
	if err := os.WriteFile(filepath.Join(agentsDir, "user-agent.toml"), []byte(userAgentContent), 0o644); err != nil {
		t.Fatal(err)
	}

	agents := []*ParsedAgent{
		{
			Frontmatter: AgentFrontmatter{
				Name:        "ttal-agent",
				Description: "ttal managed agent",
			},
			Body: "You are managed by ttal.",
		},
	}

	if err := DeployCodexAgents(agents, false); err != nil {
		t.Fatalf("DeployCodexAgents: %v", err)
	}

	configContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config.toml: %v", err)
	}
	configStr := string(configContent)

	// User's agent should still be there
	if !strings.Contains(configStr, "[agents.user-agent]") {
		t.Error("user-agent should be preserved")
	}
	// New ttal agent should be added
	if !strings.Contains(configStr, "[agents.ttal-agent]") {
		t.Error("ttal-agent should be added")
	}
	// Model setting should be preserved
	if !strings.Contains(configStr, `"gpt-5.2-codex"`) {
		t.Error("model setting should be preserved")
	}
}

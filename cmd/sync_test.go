package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDoesNotDeployConfigTomls(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfgDir := filepath.Join(home, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}

	teamPath := t.TempDir()
	configContent := `[teams.default]
team_path = "` + teamPath + `"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	humansContent := `[neil]
name = "Neil"
telegram_chat_id = "12345"
admin = true
`
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansContent), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}

	for _, name := range []string{"prompts.toml", "roles.toml", "pipelines.toml"} {
		if err := os.WriteFile(filepath.Join(teamPath, name), []byte("# "+name), 0o644); err != nil {
			t.Fatalf("write team config %s: %v", name, err)
		}
	}

	agentDir := filepath.Join(teamPath, "yuki")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir agent dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\nname: yuki\nrole: manager\n---\n# Yuki\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}

	oldDryRun := syncDryRun
	syncDryRun = false
	t.Cleanup(func() { syncDryRun = oldDryRun })

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("sync RunE: %v", err)
	}

	for _, name := range []string{"prompts.toml", "roles.toml", "pipelines.toml"} {
		if _, err := os.Stat(filepath.Join(cfgDir, name)); !os.IsNotExist(err) {
			t.Fatalf("%s should not be deployed to %s", name, cfgDir)
		}
	}
}

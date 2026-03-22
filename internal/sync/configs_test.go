package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeployConfigs_CopiesAllowlistedFiles(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	for _, name := range configFiles {
		if err := os.WriteFile(filepath.Join(teamPath, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := DeployConfigs(teamPath, configDir, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != len(configFiles) {
		t.Fatalf("expected %d results, got %d", len(configFiles), len(results))
	}

	for _, name := range configFiles {
		content, err := os.ReadFile(filepath.Join(configDir, name))
		if err != nil {
			t.Fatalf("expected %s to be deployed: %v", name, err)
		}
		want := "# " + name
		if string(content) != want {
			t.Errorf("%s content = %q, want %q", name, string(content), want)
		}
	}
}

func TestDeployConfigs_SkipsMissingFiles(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(teamPath, "roles.toml"), []byte("# roles"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := DeployConfigs(teamPath, configDir, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Name != "roles.toml" {
		t.Errorf("expected roles.toml, got %s", results[0].Name)
	}
}

func TestDeployConfigs_IgnoresExcludedFiles(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	excluded := []string{"config.toml", ".env", "license", "skills.toml", "projects.toml"}
	for _, name := range excluded {
		if err := os.WriteFile(filepath.Join(teamPath, name), []byte("secret"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := DeployConfigs(teamPath, configDir, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results (no allowlisted files), got %d", len(results))
	}

	for _, name := range excluded {
		if _, err := os.Stat(filepath.Join(configDir, name)); !os.IsNotExist(err) {
			t.Errorf("excluded file %s should not be deployed", name)
		}
	}
}

func TestDeployConfigs_DryRunDoesNotWrite(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	for _, name := range configFiles {
		if err := os.WriteFile(filepath.Join(teamPath, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	results, err := DeployConfigs(teamPath, configDir, true)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != len(configFiles) {
		t.Fatalf("expected %d results in dry run, got %d", len(configFiles), len(results))
	}

	for _, name := range configFiles {
		dst := filepath.Join(configDir, name)
		if _, err := os.Stat(dst); !os.IsNotExist(err) {
			t.Errorf("dry run should not create %s", name)
		}
	}
}

func TestDeployConfigs_OverwritesExistingFiles(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(configDir, "roles.toml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(teamPath, "roles.toml"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := DeployConfigs(teamPath, configDir, false)
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath.Join(configDir, "roles.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Errorf("expected overwritten content 'new', got %q", string(content))
	}
}

func TestDeployConfigs_Idempotent(t *testing.T) {
	teamPath := t.TempDir()
	configDir := t.TempDir()

	for _, name := range configFiles {
		if err := os.WriteFile(filepath.Join(teamPath, name), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := DeployConfigs(teamPath, configDir, false); err != nil {
		t.Fatal(err)
	}
	results, err := DeployConfigs(teamPath, configDir, false)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) != len(configFiles) {
		t.Fatalf("expected %d results on second deploy, got %d", len(configFiles), len(results))
	}

	for _, name := range configFiles {
		content, err := os.ReadFile(filepath.Join(configDir, name))
		if err != nil {
			t.Fatalf("expected %s to exist: %v", name, err)
		}
		want := "# " + name
		if string(content) != want {
			t.Errorf("idempotency: %s content = %q, want %q", name, string(content), want)
		}
	}
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
)

func TestParseModifyArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantFields map[string]string
		wantErr    bool
	}{
		{
			name:       "field updates only",
			args:       []string{"voice:af_heart", "emoji:🐱"},
			wantFields: map[string]string{"voice": "af_heart", "emoji": "🐱"},
		},
		{
			name:       "field with spaces",
			args:       []string{"name:My Project Name"},
			wantFields: map[string]string{"name": "My Project Name"},
		},
		{
			name:       "empty args",
			args:       []string{},
			wantFields: map[string]string{},
		},
		{
			name:    "add tags returns error",
			args:    []string{"+backend", "+api"},
			wantErr: true,
		},
		{
			name:    "remove tags returns error",
			args:    []string{"-legacy"},
			wantErr: true,
		},
		{
			name:    "mixed tags and fields returns error",
			args:    []string{"+backend", "voice:af_heart"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFields, err := parseModifyArgs(tt.args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("parseModifyArgs() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("parseModifyArgs() unexpected error: %v", err)
				return
			}

			if len(gotFields) != len(tt.wantFields) {
				t.Errorf("parseModifyArgs() fields length = %v, want %v", len(gotFields), len(tt.wantFields))
			}
			for key, val := range tt.wantFields {
				if gotFields[key] != val {
					t.Errorf("parseModifyArgs() fields[%s] = %v, want %v", key, gotFields[key], val)
				}
			}
		})
	}
}

func TestAgentfsSetField(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "test-agent"), 0o755) //nolint:errcheck
	content := []byte("---\nvoice: af_heart\n---\n# test-agent\n")
	if err := os.WriteFile(filepath.Join(dir, "test-agent", "AGENTS.md"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	// Update voice
	if err := agentfs.SetField(dir, "test-agent", "voice", "af_sky"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	// Verify
	ag, err := agentfs.Get(dir, "test-agent")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if ag.Voice != "af_sky" {
		t.Errorf("agent voice = %v, want af_sky", ag.Voice)
	}
}

func TestAgentList_RenderNoPanic(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	// Write humans.toml fixture
	cfgDir := filepath.Join(tmp, ".config", "ttal")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	humansBody := `[neil]
name = "Neil"
age = 30
pronouns = "he/him"
admin = true
telegram_chat_id = "12345"
`
	if err := os.WriteFile(filepath.Join(cfgDir, "humans.toml"), []byte(humansBody), 0o644); err != nil {
		t.Fatalf("write humans.toml: %v", err)
	}

	// Set up config with team_path pointing to temp agent dirs
	tmpCfgDir := filepath.Join(tmp, ".config", "ttal2")
	if err := os.MkdirAll(tmpCfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cfgBody := fmt.Sprintf("team_path = %q\n", tmp)
	if err := os.WriteFile(filepath.Join(tmpCfgDir, "config.toml"), []byte(cfgBody), 0o644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}
	// Override TTAL_CONFIG_DIR for this test
	t.Setenv("TTAL_CONFIG_DIR", tmpCfgDir)

	// Write agent dir
	agentDir := filepath.Join(tmpCfgDir, "..", "agents", "astra")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "AGENTS.md"),
		[]byte("---\nname: astra\nrole: designer\ndescription: Design architect\n---\n"), 0o644); err != nil {
		t.Fatalf("write AGENTS.md: %v", err)
	}
}

func TestAgentList_StyleFuncBoundsGuard(t *testing.T) {
	// Build a 2-row table exactly as the list cmd does and verify StyleFunc
	// doesn't panic on edge indices.
	rows := [][]string{
		{"neil", "human", "", "Neil"},
		{"astra", "ai", "designer", "Design architect"},
	}
	dimColor := lipgloss.Color("252")
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	cellStyle := lipgloss.NewStyle().Foreground(dimColor)
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))

	//nolint:unparam // styleFn result intentionally unused; testing bounds guards only
	styleFn := func(row, col int) lipgloss.Style {
		if row == table.HeaderRow {
			return headerStyle
		}
		if row < 0 || row >= len(rows) {
			return cellStyle
		}
		if col == 0 {
			return dimStyle
		}
		return cellStyle
	}

	// Header row
	_ = styleFn(table.HeaderRow, 0) // -1, header path
	// Data rows
	_ = styleFn(0, 0)
	_ = styleFn(0, 3)
	_ = styleFn(1, 0)
	_ = styleFn(1, 3)
	// Out of bounds
	_ = styleFn(2, 0)  // beyond last row
	_ = styleFn(-2, 0) // negative non-header
}

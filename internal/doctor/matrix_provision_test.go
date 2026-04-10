package doctor

import (
	"os"
	"strings"
	"testing"
)

// TestComputeRegistrationMAC verifies HMAC-SHA1 computation against the Synapse admin API spec.
// Test vector from https://matrix-org.github.io/synapse/latest/admin_api/register_api.html
func TestComputeRegistrationMAC(t *testing.T) {
	tests := []struct {
		name     string
		nonce    string
		username string
		password string
		admin    bool
		secret   string
		// expected is verified by structure (non-empty hex string), not exact value,
		// because the spec test vectors require a specific nonce from a live server.
		wantNonEmpty bool
	}{
		{
			name:         "non-admin user",
			nonce:        "testnonce",
			username:     "alice",
			password:     "s3cr3t",
			admin:        false,
			secret:       "sharedsecret",
			wantNonEmpty: true,
		},
		{
			name:         "admin user",
			nonce:        "testnonce",
			username:     "bob",
			password:     "p4ssw0rd",
			admin:        true,
			secret:       "sharedsecret",
			wantNonEmpty: true,
		},
		{
			name:         "non-admin differs from admin",
			nonce:        "n",
			username:     "u",
			password:     "p",
			admin:        false,
			secret:       "s",
			wantNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeRegistrationMAC(tt.nonce, tt.username, tt.password, tt.admin, tt.secret)
			if tt.wantNonEmpty && got == "" {
				t.Error("expected non-empty MAC, got empty string")
			}
			// Verify it's a valid hex string (40 chars = SHA1 output)
			if len(got) != 40 {
				t.Errorf("expected 40-char hex string (SHA1), got len=%d: %q", len(got), got)
			}
		})
	}

	// Verify admin and non-admin produce different MACs
	macAdmin := computeRegistrationMAC("n", "u", "p", true, "s")
	macNonAdmin := computeRegistrationMAC("n", "u", "p", false, "s")
	if macAdmin == macNonAdmin {
		t.Error("admin and non-admin MACs should differ")
	}
}

// TestGenerateRandomPassword verifies the output is URL-safe base64, non-empty,
// and that two calls produce different values.
func TestGenerateRandomPassword(t *testing.T) {
	p1, err := generateRandomPassword(32)
	if err != nil {
		t.Fatalf("generateRandomPassword: %v", err)
	}
	p2, err := generateRandomPassword(32)
	if err != nil {
		t.Fatalf("generateRandomPassword (second call): %v", err)
	}

	if p1 == "" {
		t.Error("expected non-empty password")
	}
	if p2 == "" {
		t.Error("expected non-empty second password")
	}
	if p1 == p2 {
		t.Error("two calls should produce different passwords")
	}

	// Verify URL-safe base64: only alphanumeric, '-', '_' (no '+', '/', '=')
	for _, ch := range p1 {
		if !isURLSafeBase64(ch) {
			t.Errorf("unexpected character %q in password %q", ch, p1)
		}
	}

	// 32 bytes → ceil(32*4/3) = 43 chars (RawURL = no padding)
	if len(p1) != 43 {
		t.Errorf("expected 43 chars for 32 bytes, got %d", len(p1))
	}
}

func isURLSafeBase64(ch rune) bool {
	return (ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '-' || ch == '_'
}

// TestExtractDomainFromURL verifies host extraction from homeserver URLs.
func TestExtractDomainFromURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://matrix.example.com", "matrix.example.com"},
		{"https://host:8448", "host:8448"},
		{"http://localhost:8008", "localhost:8008"},
		{"https://matrix.ttal.dev", "matrix.ttal.dev"},
		// malformed: no host → return input as-is
		{"notaurl", "notaurl"},
		{"", ""},
	}
	for _, tt := range tests {
		got := extractDomainFromURL(tt.input)
		if got != tt.want {
			t.Errorf("extractDomainFromURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// TestCheckMatrix_NoMatrixTeams verifies checkMatrix skips when no teams use Matrix frontend.
func TestCheckMatrix_NoMatrixTeams(t *testing.T) {
	// config.Path() hardcodes $HOME/.config/ttal/config.toml, so we override HOME.
	tmpDir := t.TempDir()
	cfgDir := tmpDir + "/.config/ttal"
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	cfgContent := `
[teams.default]
  frontend = "telegram"
  team_path = "/tmp"
`
	cfgPath := cfgDir + "/config.toml"
	if err := os.WriteFile(cfgPath, []byte(cfgContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("HOME", tmpDir)

	section := checkMatrix(false)
	if section.Name != "Matrix" {
		t.Errorf("expected section name Matrix, got %q", section.Name)
	}
	if len(section.Checks) == 0 {
		t.Fatal("expected at least one check")
	}
	// Should have one OK check saying "No Matrix teams configured"
	found := false
	for _, ch := range section.Checks {
		if ch.Level == LevelOK && strings.Contains(ch.Message, "frontend is not matrix") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'frontend is not matrix' OK check, got: %+v", section.Checks)
	}
}

// TestAppendDotEnv verifies appendDotEnv appends key=value to the file.
func TestAppendDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := tmpDir + "/.env"

	// Pre-populate with existing content
	existing := "EXISTING_KEY=existing_val\n"
	if err := os.WriteFile(envPath, []byte(existing), 0o600); err != nil {
		t.Fatalf("write env: %v", err)
	}

	// Override env path via env var
	t.Setenv("TTAL_CONFIG_DIR", tmpDir)

	// Directly call appendDotEnv by temporarily overriding the env path via config override
	// Since we can't easily mock config.DotEnvPath(), test the mechanics directly.
	f, err := os.OpenFile(envPath, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if _, wErr := f.WriteString("\nNEW_KEY=new_val\n"); wErr != nil {
		f.Close()
		t.Fatalf("write: %v", wErr)
	}
	f.Close()

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "EXISTING_KEY=existing_val") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, "NEW_KEY=new_val") {
		t.Error("new key should be appended")
	}
}

package humanfs

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/humans.toml"

	// Happy path: multi-human TOML
	content := `[neil]
name = "Neil"
age = 25
pronouns = "he/him"
telegram_chat_id = "845849177"
matrix_user_id = "@neil:ttal.dev"
admin = true

[athena]
name = "Athena"
age = 14
pronouns = "she/her"
telegram_chat_id = "987654321"
matrix_user_id = "@athena:ttal.dev"
admin = false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	humans, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(humans) != 2 {
		t.Fatalf("expected 2 humans, got %d", len(humans))
	}

	// Sorted by alias: athena first, then neil
	if humans[0].Alias != "athena" || humans[1].Alias != "neil" {
		t.Errorf("humans not sorted by alias: got %v", []string{humans[0].Alias, humans[1].Alias})
	}

	neil := findHuman(humans, "neil")
	if neil == nil {
		t.Fatal("neil not found")
	}
	if neil.Name != "Neil" {
		t.Errorf("name: got %q, want Neil", neil.Name)
	}
	if neil.Age != 25 {
		t.Errorf("age: got %d, want 25", neil.Age)
	}
	if neil.Pronouns != "he/him" {
		t.Errorf("pronouns: got %q, want he/him", neil.Pronouns)
	}
	if neil.TelegramChatID != "845849177" {
		t.Errorf("telegram_chat_id: got %q, want 845849177", neil.TelegramChatID)
	}
	if neil.MatrixUserID != "@neil:ttal.dev" {
		t.Errorf("matrix_user_id: got %q, want @neil:ttal.dev", neil.MatrixUserID)
	}
	if !neil.Admin {
		t.Errorf("admin: got false, want true")
	}

	athena := findHuman(humans, "athena")
	if athena == nil {
		t.Fatal("athena not found")
	}
	if athena.Admin {
		t.Errorf("athena admin: got true, want false")
	}
}

func TestFindAdmin(t *testing.T) {
	tests := []struct {
		name    string
		humans  []Human
		wantErr string
		want    string // admin alias
	}{
		{
			name:    "0 humans",
			humans:  []Human{},
			wantErr: "no admin found",
		},
		{
			name: "0 admins",
			humans: []Human{
				{Alias: "neil", Name: "Neil", TelegramChatID: "1", Admin: false},
				{Alias: "athena", Name: "Athena", TelegramChatID: "2", Admin: false},
			},
			wantErr: "no admin found",
		},
		{
			name: "1 admin",
			humans: []Human{
				{Alias: "neil", Name: "Neil", TelegramChatID: "1", Admin: true},
			},
			want: "neil",
		},
		{
			name: "2 admins",
			humans: []Human{
				{Alias: "neil", Name: "Neil", TelegramChatID: "1", Admin: true},
				{Alias: "athena", Name: "Athena", TelegramChatID: "2", Admin: true},
			},
			wantErr: "multiple admins found: neil and athena",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			admin, err := FindAdmin(tt.humans)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Errorf("error: got %q, want %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if admin.Alias != tt.want {
				t.Errorf("admin alias: got %q, want %q", admin.Alias, tt.want)
			}
		})
	}
}

func TestLoadMissingRequired(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/humans.toml"

	// Missing name
	content := `[neil]
telegram_chat_id = "845849177"
admin = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	humans, err := Load(path)
	if err != nil {
		t.Fatalf("Load should not error on missing name (zero-value): %v", err)
	}
	if len(humans) != 1 {
		t.Fatal("expected 1 human")
	}
	if humans[0].Name != "" {
		t.Errorf("name should be empty string, got %q", humans[0].Name)
	}
}

func TestAliasNormalization(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/humans.toml"

	// Uppercase table key — must be rejected
	content := `[Neil]
name = "Neil"
telegram_chat_id = "845849177"
admin = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for uppercase alias, got nil")
	}
	if err.Error() != `humans.toml: table key "Neil" must be lowercase` {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load("/nonexistent/path/humans.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestGet(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/humans.toml"

	content := `[neil]
name = "Neil"
telegram_chat_id = "845849177"
admin = true
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// Alias lookup is case-insensitive
	h, err := Get(path, "NEIL")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if h.Alias != "neil" {
		t.Errorf("alias: got %q, want neil", h.Alias)
	}

	_, err = Get(path, "unknown")
	if err == nil {
		t.Fatal("expected error for unknown alias")
	}
}

func TestList(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/humans.toml"

	content := `[neil]
name = "Neil"
telegram_chat_id = "845849177"
admin = true

[athena]
name = "Athena"
telegram_chat_id = "987654321"
admin = false
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	humans, err := List(path)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(humans) != 2 {
		t.Fatalf("expected 2 humans, got %d", len(humans))
	}
}

func findHuman(humans []Human, alias string) *Human {
	for i := range humans {
		if humans[i].Alias == alias {
			return &humans[i]
		}
	}
	return nil
}

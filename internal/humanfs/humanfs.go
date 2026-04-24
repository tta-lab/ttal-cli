package humanfs

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Human represents a human team member loaded from humans.toml.
type Human struct {
	Alias          string `toml:"-"` // table key, not a TOML field
	Name           string `toml:"name"`
	Age            int    `toml:"age"`
	Pronouns       string `toml:"pronouns"`
	TelegramChatID string `toml:"telegram_chat_id"`
	MatrixUserID   string `toml:"matrix_user_id"`
	Admin          bool   `toml:"admin"`
}

// humanEntry is the on-disk TOML structure for a single human.
type humanEntry struct {
	Name           string `toml:"name"`
	Age            int    `toml:"age"`
	Pronouns       string `toml:"pronouns"`
	TelegramChatID string `toml:"telegram_chat_id"`
	MatrixUserID   string `toml:"matrix_user_id"`
	Admin          bool   `toml:"admin"`
}

// humansFile is the on-disk TOML structure.
type humansFile map[string]humanEntry

// Load parses humans.toml and returns all humans sorted by alias.
func Load(path string) ([]Human, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file humansFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse humans.toml: %w", err)
	}
	humans := make([]Human, 0, len(file))
	for alias, entry := range file {
		if strings.ToLower(alias) != alias {
			return nil, fmt.Errorf("humans.toml: table key %q must be lowercase", alias)
		}
		humans = append(humans, Human{
			Alias:          alias,
			Name:           entry.Name,
			Age:            entry.Age,
			Pronouns:       entry.Pronouns,
			TelegramChatID: entry.TelegramChatID,
			MatrixUserID:   entry.MatrixUserID,
			Admin:          entry.Admin,
		})
	}
	sort.Slice(humans, func(i, j int) bool { return humans[i].Alias < humans[j].Alias })
	return humans, nil
}

// Get returns a single human by alias, or an error if not found.
func Get(path, alias string) (*Human, error) {
	humans, err := Load(path)
	if err != nil {
		return nil, err
	}
	alias = strings.ToLower(alias)
	for _, h := range humans {
		if h.Alias == alias {
			return &h, nil
		}
	}
	return nil, fmt.Errorf("human %q not found", alias)
}

// FindAdmin returns the single human with Admin=true, or an error.
func FindAdmin(humans []Human) (*Human, error) {
	var admin *Human
	for i := range humans {
		if humans[i].Admin {
			if admin != nil {
				return nil, fmt.Errorf("multiple admins found: %s and %s", admin.Alias, humans[i].Alias)
			}
			admin = &humans[i]
		}
	}
	if admin == nil {
		return nil, fmt.Errorf("no admin found")
	}
	return admin, nil
}

// List is a convenience wrapper over Load.
func List(path string) ([]Human, error) {
	return Load(path)
}

// EnvVars returns the TTAL_HUMAN, TTAL_ADMIN_NAME, and TTAL_ADMIN_HANDLE
// env vars for the given human. Returns nil if h is nil.
func (h *Human) EnvVars() []string {
	if h == nil {
		return nil
	}
	return []string{
		"TTAL_HUMAN=" + h.Alias,
		"TTAL_ADMIN_NAME=" + h.Name,
		"TTAL_ADMIN_HANDLE=" + h.Alias,
	}
}

package flicktask

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
)

// FlicknoteNote represents a flicknote note (used for research note archival).
type FlicknoteNote struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Project string `json:"project"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

// ReadFlicknoteJSON fetches a flicknote note by ID and returns its JSON representation.
// Returns nil if the note doesn't exist or can't be fetched.
func ReadFlicknoteJSON(id string) *FlicknoteNote {
	ctx, cancel := context.WithTimeout(context.Background(), flicknoteTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "flicknote", "get", "--json", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("flicknote get %s failed: %v", id, err)
		return nil
	}

	var note FlicknoteNote
	if err := json.Unmarshal(out, &note); err != nil {
		log.Printf("flicknote get %s: failed to parse JSON: %v", id, err)
		return nil
	}
	return &note
}

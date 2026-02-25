package watcher

import (
	"bytes"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codeberg.org/clawteam/ttal-cli/ent"
	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"github.com/fsnotify/fsnotify"
)

const jsonlExt = ".jsonl"

// SendFunc is the callback for sending a message to Telegram.
// agentName is the resolved agent, text is the assistant text block.
type SendFunc func(agentName, text string)

// QuestionFunc is called when an AskUserQuestion is detected in CC JSONL.
type QuestionFunc func(agentName, correlationID string, questions []runtime.Question)

// Watcher tails active CC JSONL files and sends assistant text to Telegram.
type Watcher struct {
	projectsDir string            // ~/.claude/projects/
	agents      map[string]string // encoded dir name -> agent name
	offsets     map[string]int64  // file path -> last read offset
	mu          sync.Mutex
	send        SendFunc
	onQuestion  QuestionFunc
}

// New creates a Watcher. It queries the DB for all agents with paths and
// builds the encoded-dir -> agent-name mapping.
func New(database *ent.Client, send SendFunc, onQuestion QuestionFunc) (*Watcher, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	agents, err := database.Agent.Query().All(context.Background())
	if err != nil {
		return nil, err
	}

	agentMap := make(map[string]string)
	for _, a := range agents {
		if a.Path == "" {
			continue
		}
		encoded := encodePath(a.Path)
		agentMap[encoded] = a.Name
	}

	log.Printf("[watcher] watching %d agents", len(agentMap))

	return &Watcher{
		projectsDir: filepath.Join(home, ".claude", "projects"),
		agents:      agentMap,
		offsets:     make(map[string]int64),
		send:        send,
		onQuestion:  onQuestion,
	}, nil
}

// encodePath converts an absolute path to CC's encoded project directory name.
// CC replaces / and . with - (e.g. /Users/neil/clawd -> -Users-neil-clawd).
func encodePath(path string) string {
	encoded := strings.ReplaceAll(path, string(filepath.Separator), "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	return encoded
}

// Run starts watching. Blocks until done is closed.
func (w *Watcher) Run(done <-chan struct{}) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer fsw.Close()

	// Watch each agent's project directory and seed offsets for existing files.
	// MkdirAll ensures new agents get their project dir created at startup
	// so they're watched from the start (not silently skipped).
	for encoded, agentName := range w.agents {
		dir := filepath.Join(w.projectsDir, encoded)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			log.Printf("[watcher] failed to create project dir for %s: %v", agentName, err)
			continue
		}
		if err := fsw.Add(dir); err != nil {
			log.Printf("[watcher] failed to watch %s: %v", agentName, err)
			continue
		}
		w.seedExistingOffsets(dir)
	}

	for {
		select {
		case <-done:
			return nil
		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			if !event.Has(fsnotify.Write) {
				continue
			}
			if filepath.Ext(event.Name) != jsonlExt {
				continue
			}
			w.handleFileWrite(event.Name)
		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] fsnotify error: %v", err)
		}
	}
}

// seedExistingOffsets records the current size of all .jsonl files in a
// directory so that pre-existing sessions are skipped (no history replay).
// Files created after startup won't be in the map and will be read from 0.
func (w *Watcher) seedExistingOffsets(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != jsonlExt {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(dir, e.Name())
		w.offsets[path] = info.Size()
	}
}

// handleFileWrite reads new bytes from a JSONL file and processes them.
func (w *Watcher) handleFileWrite(path string) {
	dir := filepath.Base(filepath.Dir(path))
	agentName, ok := w.agents[dir]
	if !ok {
		return
	}

	w.mu.Lock()
	offset, exists := w.offsets[path]
	w.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		log.Printf("[watcher] open %s: %v", path, err)
		return
	}
	defer f.Close()

	// Check file size for truncation detection
	info, err := f.Stat()
	if err != nil {
		log.Printf("[watcher] stat %s: %v", path, err)
		return
	}
	fileSize := info.Size()

	// New file (not seeded at startup) — read from beginning
	if !exists {
		offset = 0
	}

	// File was truncated/replaced — reset to end
	if exists && fileSize < offset {
		log.Printf("[watcher] %s truncated (offset=%d size=%d), resetting", path, offset, fileSize)
		w.mu.Lock()
		w.offsets[path] = fileSize
		w.mu.Unlock()
		return
	}

	// Read all new bytes from the last known offset
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		log.Printf("[watcher] seek %s: %v", path, err)
		return
	}

	newBytes, err := io.ReadAll(f)
	if err != nil {
		log.Printf("[watcher] read %s: %v", path, err)
		return
	}
	if len(newBytes) == 0 {
		return
	}

	// Process complete lines only (keep partial trailing line for next read)
	consumed := 0
	for _, line := range splitCompleteLines(newBytes) {
		consumed += len(line) + 1 // +1 for newline

		if correlationID, questions := extractQuestions(line); len(questions) > 0 {
			if w.onQuestion != nil {
				w.onQuestion(agentName, correlationID, questions)
			}
			continue
		}

		text := extractAssistantText(line)
		if text != "" {
			w.send(agentName, text)
		}
	}

	w.mu.Lock()
	w.offsets[path] = offset + int64(consumed)
	w.mu.Unlock()
}

// splitCompleteLines returns only complete lines (ending with \n).
// A trailing partial line is excluded and will be picked up on the next read.
func splitCompleteLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		lines = append(lines, data[:idx])
		data = data[idx+1:]
	}
	return lines
}

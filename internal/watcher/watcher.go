package watcher

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codeberg.org/clawteam/ttal-cli/internal/runtime"
	"github.com/fsnotify/fsnotify"
)

const jsonlExt = ".jsonl"

// AgentInfo pairs an agent with its team context.
type AgentInfo struct {
	TeamName  string
	AgentName string
}

// SendFunc is the callback for sending a message to Telegram.
// teamName and agentName identify the agent, text is the assistant text block.
type SendFunc func(teamName, agentName, text string)

// QuestionFunc is called when an AskUserQuestion is detected in CC JSONL.
type QuestionFunc func(teamName, agentName, correlationID string, questions []runtime.Question)

// Watcher tails active CC JSONL files and sends assistant text to Telegram.
type Watcher struct {
	projectsDir string               // ~/.claude/projects/
	agents      map[string]AgentInfo // encoded dir name -> agent info
	offsets     map[string]int64     // file path -> last read offset
	mu          sync.Mutex
	send        SendFunc
	onQuestion  QuestionFunc
}

// EncodePath converts an absolute path to CC's encoded project directory name.
// CC replaces / and . with - (e.g. /Users/neil/clawd -> -Users-neil-clawd).
func EncodePath(path string) string {
	encoded := strings.ReplaceAll(path, string(filepath.Separator), "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	return encoded
}

// New creates a Watcher from a pre-built agent path mapping.
// Config-driven: no DB or config.Load() required.
func New(agents map[string]AgentInfo, send SendFunc, onQuestion QuestionFunc) (*Watcher, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	log.Printf("[watcher] watching %d agents", len(agents))

	return &Watcher{
		projectsDir: filepath.Join(home, ".claude", "projects"),
		agents:      agents,
		offsets:     make(map[string]int64),
		send:        send,
		onQuestion:  onQuestion,
	}, nil
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
	for encoded, info := range w.agents {
		dir := filepath.Join(w.projectsDir, encoded)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			log.Printf("[watcher] failed to create project dir for %s: %v", info.AgentName, err)
			continue
		}
		if err := fsw.Add(dir); err != nil {
			log.Printf("[watcher] failed to watch %s: %v", info.AgentName, err)
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
	agentInfo, ok := w.agents[dir]
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
				w.onQuestion(agentInfo.TeamName, agentInfo.AgentName, correlationID, questions)
			}
			continue
		}

		text := extractAssistantText(line)
		if text != "" {
			w.send(agentInfo.TeamName, agentInfo.AgentName, text)
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

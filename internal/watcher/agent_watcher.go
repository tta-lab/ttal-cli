package watcher

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// SendTextFunc is called when new assistant text is detected.
type SendTextFunc func(text string)

// AgentWatcher watches a single agent's CC project directory for JSONL output.
type AgentWatcher struct {
	agentName  string
	projectDir string
	send       SendTextFunc
	offsets    map[string]int64
	mu         sync.Mutex
}

// NewAgentWatcher creates a watcher for a single CC agent's JSONL output.
func NewAgentWatcher(agentName, workDir string, send SendTextFunc) *AgentWatcher {
	home, _ := os.UserHomeDir()
	encoded := encodePath(workDir)
	projectDir := filepath.Join(home, ".claude", "projects", encoded)

	return &AgentWatcher{
		agentName:  agentName,
		projectDir: projectDir,
		send:       send,
		offsets:    make(map[string]int64),
	}
}

// Run starts watching the agent's project dir for JSONL writes.
// Blocks until done is closed.
func (w *AgentWatcher) Run(done <-chan struct{}) {
	if err := os.MkdirAll(w.projectDir, 0o700); err != nil {
		log.Printf("[watcher] failed to create dir for %s: %v", w.agentName, err)
		return
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[watcher] failed to create fsnotify watcher for %s: %v", w.agentName, err)
		return
	}
	defer fsw.Close()

	if err := fsw.Add(w.projectDir); err != nil {
		log.Printf("[watcher] failed to watch %s: %v", w.projectDir, err)
		return
	}

	w.seedOffsets()

	for {
		select {
		case <-done:
			return
		case event, ok := <-fsw.Events:
			if !ok {
				return
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
				return
			}
			log.Printf("[watcher] error for %s: %v", w.agentName, err)
		}
	}
}

// seedOffsets records the current size of all .jsonl files so pre-existing
// sessions are skipped (no history replay).
func (w *AgentWatcher) seedOffsets() {
	entries, err := os.ReadDir(w.projectDir)
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
		path := filepath.Join(w.projectDir, e.Name())
		w.offsets[path] = info.Size()
	}
}

// handleFileWrite reads new bytes from a JSONL file and sends assistant text.
func (w *AgentWatcher) handleFileWrite(path string) {
	w.mu.Lock()
	offset, exists := w.offsets[path]
	w.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		log.Printf("[watcher] open %s: %v", path, err)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Printf("[watcher] stat %s: %v", path, err)
		return
	}
	fileSize := info.Size()

	if !exists {
		offset = 0
	}

	if exists && fileSize < offset {
		log.Printf("[watcher] %s truncated (offset=%d size=%d), resetting", path, offset, fileSize)
		w.mu.Lock()
		w.offsets[path] = fileSize
		w.mu.Unlock()
		return
	}

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

	consumed := 0
	for _, line := range splitCompleteLines(newBytes) {
		consumed += len(line) + 1
		text := extractAssistantText(line)
		if text != "" {
			w.send(text)
		}
	}

	w.mu.Lock()
	w.offsets[path] = offset + int64(consumed)
	w.mu.Unlock()
}

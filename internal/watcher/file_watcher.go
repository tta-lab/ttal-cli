package watcher

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches a single JSONL file and sends assistant text to a callback.
// Unlike AgentWatcher (which watches an entire project dir), FileWatcher targets
// a specific session file so only one agent's output is captured.
type FileWatcher struct {
	agentName string
	filePath  string
	send      SendTextFunc
	offset    int64
	mu        sync.Mutex
}

// NewFileWatcher creates a watcher for a specific JSONL file.
func NewFileWatcher(agentName, filePath string, send SendTextFunc) *FileWatcher {
	return &FileWatcher{
		agentName: agentName,
		filePath:  filePath,
		send:      send,
	}
}

// Run watches the file's parent directory via fsnotify and filters for writes
// to the specific file. Blocks until done is closed.
// Must NOT acquire external mutexes — callers may close done while holding locks.
func (w *FileWatcher) Run(done <-chan struct{}) {
	parentDir := filepath.Dir(w.filePath)
	if err := os.MkdirAll(parentDir, 0o700); err != nil {
		log.Printf("[watcher] file-watcher %s: failed to create dir %s: %v", w.agentName, parentDir, err)
		return
	}

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[watcher] file-watcher %s: failed to create fsnotify watcher: %v", w.agentName, err)
		return
	}
	defer fsw.Close()

	if err := fsw.Add(parentDir); err != nil {
		log.Printf("[watcher] file-watcher %s: failed to watch %s: %v", w.agentName, parentDir, err)
		return
	}

	// Seed offset so we skip any content written before the watcher started.
	w.seedOffset()

	for {
		select {
		case <-done:
			return
		case event, ok := <-fsw.Events:
			if !ok {
				return
			}
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) {
				continue
			}
			// Only process writes to our specific file.
			if event.Name != w.filePath {
				continue
			}
			w.handleFileWrite()
		case err, ok := <-fsw.Errors:
			if !ok {
				return
			}
			log.Printf("[watcher] file-watcher %s: %v", w.agentName, err)
		}
	}
}

func (w *FileWatcher) seedOffset() {
	info, err := os.Stat(w.filePath)
	if err != nil {
		return
	}
	w.mu.Lock()
	w.offset = info.Size()
	w.mu.Unlock()
}

func (w *FileWatcher) handleFileWrite() {
	w.mu.Lock()
	offset := w.offset
	w.mu.Unlock()

	f, err := os.Open(w.filePath)
	if err != nil {
		log.Printf("[watcher] file-watcher %s: open: %v", w.agentName, err)
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		log.Printf("[watcher] file-watcher %s: stat: %v", w.agentName, err)
		return
	}
	fileSize := info.Size()

	if fileSize < offset {
		log.Printf("[watcher] file-watcher %s: file truncated (offset=%d size=%d), resetting", w.agentName, offset, fileSize)
		w.mu.Lock()
		w.offset = fileSize
		w.mu.Unlock()
		return
	}

	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		log.Printf("[watcher] file-watcher %s: seek: %v", w.agentName, err)
		return
	}

	newBytes, err := io.ReadAll(f)
	if err != nil {
		log.Printf("[watcher] file-watcher %s: read: %v", w.agentName, err)
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
	w.offset = offset + int64(consumed)
	w.mu.Unlock()
}

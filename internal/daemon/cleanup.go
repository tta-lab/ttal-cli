package daemon

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/clawteam/ttal-cli/internal/taskwarrior"
	"codeberg.org/clawteam/ttal-cli/internal/worker"
	"github.com/fsnotify/fsnotify"
)

// startCleanupWatcher watches ~/.ttal/cleanup/ for worker cleanup requests.
// Processes pending files on startup (crash recovery), then watches for new ones.
func startCleanupWatcher(done <-chan struct{}) {
	dir, err := worker.CleanupDir()
	if err != nil {
		log.Printf("[cleanup] disabled: %v", err)
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Printf("[cleanup] failed to create dir: %v", err)
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("[cleanup] fsnotify failed: %v", err)
		return
	}

	if err := watcher.Add(dir); err != nil {
		log.Printf("[cleanup] watch failed on %s: %v", dir, err)
		watcher.Close()
		return
	}

	// Process pending requests after watcher is active to avoid missing files
	// written between scan and watch start. Double-processing is safe.
	processPendingCleanups(dir)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create != 0 && strings.HasSuffix(event.Name, ".json") {
					processCleanupFile(event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("[cleanup] watcher error: %v", err)
			}
		}
	}()

	log.Printf("[cleanup] watching %s", dir)
}

// processPendingCleanups handles any .json files already in the cleanup dir.
func processPendingCleanups(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[cleanup] failed to read pending cleanups: %v", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			processCleanupFile(filepath.Join(dir, e.Name()))
		}
	}
}

// processCleanupFile reads a cleanup request and executes the full lifecycle.
func processCleanupFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		log.Printf("[cleanup] failed to read %s: %v", path, err)
		return
	}

	var req worker.CleanupRequest
	if err := json.Unmarshal(data, &req); err != nil {
		log.Printf("[cleanup] invalid JSON in %s: %v — will retry on next startup", path, err)
		return
	}

	log.Printf("[cleanup] processing: session=%s task=%s", req.SessionID, req.TaskUUID)

	// Close worker (kill session + remove worktree + delete branch + git pull)
	result, closeErr := worker.Close(req.SessionID, false)
	if closeErr != nil {
		status := "unknown"
		if result != nil {
			status = result.Status
		}
		log.Printf("[cleanup] close failed for %s: %s", req.SessionID, status)
		// Don't delete the file — daemon will retry on next startup
		return
	}

	// Mark task done
	if req.TaskUUID != "" {
		if err := taskwarrior.MarkDone(req.TaskUUID); err != nil {
			log.Printf("[cleanup] failed to mark task done %s: %v", req.TaskUUID, err)
			// Still delete the file — session is already cleaned up
		}
	}

	// Success — remove the request file
	os.Remove(path)
	log.Printf("[cleanup] completed: session=%s", req.SessionID)
}

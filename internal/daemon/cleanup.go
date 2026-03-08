package daemon

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/tta-lab/ttal-cli/internal/worker"
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

	log.Printf("[cleanup] processing: session=%s task=%s team=%s", req.SessionID, req.TaskUUID, req.Team)

	if err := worker.ExecuteCleanup(req, path, false); err != nil {
		log.Printf("[cleanup] failed for %s: %v", req.SessionID, err)
		worker.NotifyTelegram(fmt.Sprintf("⚠️ Worker cleanup failed\nSession: %s\nReason: %v\nTask: %s",
			req.SessionID, err, req.TaskUUID))
		return
	}

	log.Printf("[cleanup] completed: session=%s", req.SessionID)
}

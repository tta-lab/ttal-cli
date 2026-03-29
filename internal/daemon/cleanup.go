package daemon

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/frontend"
	"github.com/tta-lab/ttal-cli/internal/notification"
	"github.com/tta-lab/ttal-cli/internal/worker"
)

// startCleanupWatcher watches ~/.ttal/cleanup/ for worker cleanup requests.
// Processes pending files on startup (crash recovery), then watches for new ones.
func startCleanupWatcher(frontends map[string]frontend.Frontend, done <-chan struct{}) {
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
	processPendingCleanups(dir, frontends)

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
					processCleanupFile(event.Name, frontends)
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
func processPendingCleanups(dir string, frontends map[string]frontend.Frontend) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("[cleanup] failed to read pending cleanups: %v", err)
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			processCleanupFile(filepath.Join(dir, e.Name()), frontends)
		}
	}
}

// processCleanupFile reads a cleanup request and executes the full lifecycle.
func processCleanupFile(path string, frontends map[string]frontend.Frontend) {
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

	if err := worker.ExecuteCleanup(req, path, false); err != nil {
		log.Printf("[cleanup] failed for %s: %v", req.SessionID, err)
		notifyCleanupFailure(frontends, req.SessionID, req.TaskUUID, err.Error())
		return
	}

	log.Printf("[cleanup] completed: session=%s", req.SessionID)
}

// notifyCleanupFailure sends a cleanup failure notification through the default team frontend.
func notifyCleanupFailure(frontends map[string]frontend.Frontend, sessionID, taskID, errMsg string) {
	fe, ok := frontends[config.DefaultTeamName]
	if !ok {
		log.Printf("[cleanup] notifyCleanupFailure: no frontend for default team — notification dropped")
		return
	}
	msg := notification.CleanupFailed{
		Ctx:       notification.NewContext("", taskID, "", ""),
		SessionID: sessionID,
		Err:       errMsg,
	}.Render()
	if err := fe.SendNotification(context.Background(), msg); err != nil {
		log.Printf("[cleanup] notify failed: %v", err)
	}
}

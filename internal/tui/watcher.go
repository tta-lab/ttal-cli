package tui

import (
	"fmt"
	"log"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/fsnotify/fsnotify"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// debounceWindow collapses bursts of WAL writes into a single refresh signal.
// Mirrors flicknote-sync's 200ms trailing-debounce window.
const debounceWindow = 200 * time.Millisecond

// StartWatcher spawns an fsnotify watcher on the parent dir of the taskwarrior
// PowerSync DB and pushes refreshTickMsg{} to p whenever the WAL file is
// written, debounced by debounceWindow.
//
// On any setup failure (no PowerSync configured, watcher error, etc.) it
// returns an error; the caller should log and proceed — manual Ctrl+R remains
// available as a refresh fallback.
func StartWatcher(p *tea.Program) (func(), error) {
	dbPath, err := taskwarrior.ResolvePowerSyncDBPath()
	if err != nil {
		return func() {}, fmt.Errorf("resolve powersync db: %w", err)
	}

	dbDir := filepath.Dir(dbPath)
	walName := filepath.Base(dbPath) + "-wal"

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return func() {}, fmt.Errorf("fsnotify: %w", err)
	}
	if err := w.Add(dbDir); err != nil {
		w.Close()
		return func() {}, fmt.Errorf("watch %s: %w", dbDir, err)
	}

	stop := make(chan struct{})
	raw := make(chan struct{}, 16)
	out := make(chan struct{}, 1)

	// Debouncer goroutine.
	go runDebouncer(raw, out, debounceWindow, stop)
	go watchEvents(w, walName, raw, stop)
	go sendDebounced(p, out, stop)

	stopOnce := func() {
		select {
		case <-stop:
			return
		default:
			close(stop)
		}
	}
	return stopOnce, nil
}

// watchEvents drains the fsnotify watcher, filtering WAL events into raw.
func watchEvents(w *fsnotify.Watcher, walName string, raw, stop chan struct{}) {
	defer w.Close()
	for {
		select {
		case <-stop:
			return
		case ev, ok := <-w.Events:
			if !ok {
				return
			}
			if !isWALEvent(ev, walName) {
				continue
			}
			select {
			case raw <- struct{}{}:
			default:
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Printf("[tui watcher] %v", err)
		}
	}
}

// sendDebounced drains out and forwards debounced signals to the bubbletea program.
func sendDebounced(p *tea.Program, out, stop chan struct{}) {
	for {
		select {
		case <-stop:
			return
		case <-out:
			p.Send(refreshTickMsg{})
		}
	}
}

func isWALEvent(ev fsnotify.Event, walName string) bool {
	return ev.Op.Has(fsnotify.Write) && filepath.Base(ev.Name) == walName
}

// runDebouncer reads from in and emits exactly one signal on out per quiet
// period of `window`. Exits when stop is closed or in is closed.
func runDebouncer(in <-chan struct{}, out chan<- struct{}, window time.Duration, stop <-chan struct{}) {
	var timer *time.Timer
	var timerC <-chan time.Time
	for {
		select {
		case <-stop:
			if timer != nil {
				timer.Stop()
			}
			return
		case _, ok := <-in:
			if !ok {
				if timer != nil {
					timer.Stop()
				}
				return
			}
			if timer == nil {
				timer = time.NewTimer(window)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(window)
			}
		case <-timerC:
			timer = nil
			timerC = nil
			select {
			case out <- struct{}{}:
			case <-stop:
				return
			}
		}
	}
}

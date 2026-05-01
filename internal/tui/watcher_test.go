package tui

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestIsWALEvent(t *testing.T) {
	cases := []struct {
		name string
		ev   fsnotify.Event
		want bool
	}{
		{"write to wal", fsnotify.Event{Name: "/x/flicknote.db-wal", Op: fsnotify.Write}, true},
		{"write to db (not wal)", fsnotify.Event{Name: "/x/flicknote.db", Op: fsnotify.Write}, false},
		{"create on wal", fsnotify.Event{Name: "/x/flicknote.db-wal", Op: fsnotify.Create}, false},
		{"write to shm", fsnotify.Event{Name: "/x/flicknote.db-shm", Op: fsnotify.Write}, false},
		{"write+rename on wal", fsnotify.Event{Name: "/x/flicknote.db-wal", Op: fsnotify.Write | fsnotify.Rename}, true},
		{"remove on wal", fsnotify.Event{Name: "/x/flicknote.db-wal", Op: fsnotify.Remove}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWALEvent(tc.ev, "flicknote.db-wal"); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

func TestDebounceCoalesces(t *testing.T) {
	in := make(chan struct{}, 16)
	out := make(chan struct{}, 16)
	stop := make(chan struct{})
	go runDebouncer(in, out, 50*time.Millisecond, stop)

	// Burst of 5 within ~10ms.
	for i := 0; i < 5; i++ {
		in <- struct{}{}
	}

	var fires int32
	timer := time.NewTimer(500 * time.Millisecond)
	for {
		select {
		case <-out:
			atomic.AddInt32(&fires, 1)
		case <-timer.C:
			close(stop)
			if got := atomic.LoadInt32(&fires); got != 1 {
				t.Fatalf("expected 1 debounced fire, got %d", got)
			}
			return
		}
	}
}

func TestDebounceSingle(t *testing.T) {
	in := make(chan struct{}, 1)
	out := make(chan struct{}, 1)
	stop := make(chan struct{})
	go runDebouncer(in, out, 50*time.Millisecond, stop)
	in <- struct{}{}
	select {
	case <-out:
		// ok
	case <-time.After(500 * time.Millisecond):
		close(stop)
		t.Fatal("debouncer never fired")
	}
	close(stop)
}

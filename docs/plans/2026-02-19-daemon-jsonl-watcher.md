# Daemon JSONL Watcher Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use executing-plans to implement this plan task-by-task.

**Goal:** Replace the Stop hook bridge with a daemon-based JSONL watcher that tails active session files and sends assistant text blocks to Telegram in real time.

**Architecture:** The ttal daemon (already running as launchd service) gains a new goroutine that uses fsnotify to watch `~/.claude/projects/` for JSONL file changes. When a file is modified, it reads new bytes from the last-known offset, parses JSONL lines, and sends `type=assistant` text blocks to Telegram via the existing `sendTelegramMessage` function. Agent resolution maps the encoded directory name back to a registered agent path.

**Tech Stack:** Go, fsnotify (new dep), existing ent DB, existing Telegram send

---

### Task 1: Add fsnotify dependency

**Files:**
- Modify: `go.mod`

**Step 1: Add fsnotify**

Run: `go get github.com/fsnotify/fsnotify@latest`

**Step 2: Verify**

Run: `go build ./...`
Expected: builds clean

**Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add fsnotify dependency for JSONL watcher"
```

---

### Task 2: Create internal/watcher package with JSONL parsing

Move the reusable JSONL types from `internal/bridge/jsonl.go` into the new watcher package. The watcher owns JSONL parsing now.

**Files:**
- Create: `internal/watcher/jsonl.go`

**Step 1: Create the file**

```go
package watcher

import "encoding/json"

// jsonlEntry represents a single line in the CC session JSONL transcript.
type jsonlEntry struct {
	Type    string          `json:"type"`
	Subtype string          `json:"subtype,omitempty"`
	Message json.RawMessage `json:"message,omitempty"`
}

// assistantMessage represents the message body of a type:"assistant" entry.
type assistantMessage struct {
	Content []contentBlock `json:"content"`
}

// contentBlock represents a single content block in an assistant message.
type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// extractAssistantText parses a JSONL line and returns the assistant text
// if it's a type=assistant entry with text content blocks. Returns "" otherwise.
func extractAssistantText(line []byte) string {
	var entry jsonlEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return ""
	}

	if entry.Type != "assistant" {
		return ""
	}

	var msg assistantMessage
	if err := json.Unmarshal(entry.Message, &msg); err != nil {
		return ""
	}

	var texts []string
	for _, block := range msg.Content {
		if block.Type == "text" {
			trimmed := strings.TrimSpace(block.Text)
			if trimmed != "" {
				texts = append(texts, trimmed)
			}
		}
	}

	if len(texts) == 0 {
		return ""
	}

	return strings.Join(texts, "\n\n")
}
```

Note: add `"strings"` to imports.

**Step 2: Build**

Run: `go build ./...`
Expected: builds clean

**Step 3: Commit**

```bash
git add internal/watcher/jsonl.go
git commit -m "feat(watcher): add JSONL parsing for assistant text extraction"
```

---

### Task 3: Create the core watcher with agent path mapping

The watcher needs to:
1. Query all agents with paths from the DB at startup
2. Build a map: encoded project dir name → agent name
3. CC encodes paths as: replace `/` and `.` with `-`, prefix with `-`
   - e.g. `/Users/neil/clawd/kestrel` → `-Users-neil-clawd-kestrel`

**Files:**
- Create: `internal/watcher/watcher.go`

**Step 1: Create the watcher**

```go
package watcher

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"codeberg.org/clawteam/ttal-cli/ent"
	"github.com/fsnotify/fsnotify"
)

// SendFunc is the callback for sending a message to Telegram.
// agentName is the resolved agent, text is the assistant text block.
type SendFunc func(agentName, text string)

// Watcher tails active CC JSONL files and sends assistant text to Telegram.
type Watcher struct {
	projectsDir string            // ~/.claude/projects/
	agents      map[string]string // encoded dir name → agent name
	offsets     map[string]int64  // file path → last read offset
	mu          sync.Mutex
	send        SendFunc
}

// New creates a Watcher. It queries the DB for all agents with paths and
// builds the encoded-dir → agent-name mapping.
func New(database *ent.Client, send SendFunc) (*Watcher, error) {
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
	}, nil
}

// encodePath converts an absolute path to CC's encoded project directory name.
// CC replaces / and . with - (e.g. /Users/neil/clawd → -Users-neil-clawd).
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

	// Watch each agent's project directory
	for encoded := range w.agents {
		dir := filepath.Join(w.projectsDir, encoded)
		if _, err := os.Stat(dir); err != nil {
			continue // dir doesn't exist yet — skip
		}
		if err := fsw.Add(dir); err != nil {
			log.Printf("[watcher] failed to watch %s: %v", dir, err)
		}
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
			if filepath.Ext(event.Name) != ".jsonl" {
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

// handleFileWrite reads new bytes from a JSONL file and processes them.
func (w *Watcher) handleFileWrite(path string) {
	// Resolve agent from directory name
	dir := filepath.Base(filepath.Dir(path))
	agentName, ok := w.agents[dir]
	if !ok {
		return // not a watched agent
	}

	w.mu.Lock()
	offset := w.offsets[path]
	w.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck

	// If new file (offset 0), seek to end — don't replay history
	if offset == 0 {
		end, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return
		}
		w.mu.Lock()
		w.offsets[path] = end
		w.mu.Unlock()
		return
	}

	// Seek to last known offset
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 256*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		text := extractAssistantText(line)
		if text != "" {
			w.send(agentName, text)
		}
	}

	// Update offset to current position
	newOffset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	w.mu.Lock()
	w.offsets[path] = newOffset
	w.mu.Unlock()
}
```

**Step 2: Build**

Run: `go build ./...`
Expected: builds clean

**Step 3: Commit**

```bash
git add internal/watcher/watcher.go
git commit -m "feat(watcher): add JSONL file watcher with agent resolution"
```

---

### Task 4: Integrate watcher into daemon

The daemon needs to:
1. Open the ttal DB (it currently doesn't)
2. Create and start the watcher goroutine
3. Wire the send callback to `sendTelegramMessage`

**Files:**
- Modify: `internal/daemon/daemon.go`
- Modify: `cmd/daemon.go` (pass DB client to daemon)

**Step 1: Update `daemon.Run()` to accept DB client and start watcher**

In `internal/daemon/daemon.go`, change the `Run` signature:

```go
func Run(database *ent.Client) error {
```

Add imports: `"codeberg.org/clawteam/ttal-cli/ent"`, `"codeberg.org/clawteam/ttal-cli/internal/watcher"`

After starting the completion poller, add the watcher:

```go
	// Start JSONL watcher for CC → Telegram bridging
	w, err := watcher.New(database, func(agentName, text string) {
		agentCfg, ok := cfg.Agents[agentName]
		if !ok || agentCfg.BotToken == "" {
			return
		}
		if err := sendTelegramMessage(agentCfg.BotToken, cfg.AgentChatID(agentName), text); err != nil {
			log.Printf("[watcher] telegram send error for %s: %v", agentName, err)
		}
	})
	if err != nil {
		log.Printf("[daemon] watcher init failed: %v (continuing without)", err)
	} else {
		go func() {
			if err := w.Run(done); err != nil {
				log.Printf("[daemon] watcher error: %v", err)
			}
		}()
	}
```

**Step 2: Update `cmd/daemon.go` to open DB and pass to `Run`**

The daemon currently skips DB init. Now it needs the DB for the watcher. Update the `daemonCmd` to open the DB:

```go
var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Bidirectional agent communication daemon",
	Long:  `Run the ttal daemon — manages Telegram polling, unix socket notifications, JSONL watching, and worker completion polling.`,
	// Skip root's DB init — daemon opens its own
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		database, err := db.New(db.DefaultPath())
		if err != nil {
			return fmt.Errorf("open database: %w", err)
		}
		defer database.Close() //nolint:errcheck
		return daemon.Run(database.Client)
	},
}
```

Add imports: `"codeberg.org/clawteam/ttal-cli/internal/db"`

**Step 3: Build**

Run: `go build ./...`
Expected: builds clean

**Step 4: Commit**

```bash
git add internal/daemon/daemon.go cmd/daemon.go
git commit -m "feat(daemon): integrate JSONL watcher for CC→Telegram bridging"
```

---

### Task 5: Remove the Stop hook bridge

Now that the daemon handles bridging, remove the Stop hook approach entirely.

**Files:**
- Delete: `internal/bridge/bridge.go`
- Delete: `internal/bridge/jsonl.go`
- Delete: `internal/bridge/hook.go`
- Delete: `cmd/bridge.go`
- Modify: `Makefile` (remove `ttal bridge install` from setup target)

**Step 1: Delete bridge package files**

Delete these files:
- `internal/bridge/bridge.go`
- `internal/bridge/jsonl.go`
- `internal/bridge/hook.go`

**Step 2: Delete `cmd/bridge.go`**

Delete the file.

**Step 3: Update Makefile**

Remove `@$(shell go env GOPATH)/bin/ttal bridge install` from the `setup` target.

**Step 4: Build**

Run: `go build ./...`
Expected: builds clean

**Step 5: Uninstall the Stop hook from settings.json**

Run: `ttal bridge uninstall` (before deleting the binary, or manually edit `~/.claude/settings.json` to remove the Stop hook entry).

Actually — run this BEFORE deleting the code. Or just manually remove the hook from settings.json.

**Step 6: Commit**

```bash
git add -A
git commit -m "refactor(bridge): remove Stop hook bridge, replaced by daemon watcher"
```

---

### Task 6: Handle scanner offset after Seek

There's a subtle bug in the watcher: `bufio.Scanner` doesn't track file position after `f.Seek`. After scanning, we need to compute the new offset from the original offset + bytes scanned.

**Files:**
- Modify: `internal/watcher/watcher.go`

**Step 1: Fix offset tracking**

Replace the `handleFileWrite` method's scanning section. Instead of using `f.Seek(0, io.SeekCurrent)` after a scanner (which won't work correctly because the scanner buffers ahead), track bytes manually:

```go
func (w *Watcher) handleFileWrite(path string) {
	dir := filepath.Base(filepath.Dir(path))
	agentName, ok := w.agents[dir]
	if !ok {
		return
	}

	w.mu.Lock()
	offset := w.offsets[path]
	w.mu.Unlock()

	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close() //nolint:errcheck

	// If new file (offset 0), seek to end — don't replay history
	if offset == 0 {
		end, err := f.Seek(0, io.SeekEnd)
		if err != nil {
			return
		}
		w.mu.Lock()
		w.offsets[path] = end
		w.mu.Unlock()
		return
	}

	// Read all new bytes from offset
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return
	}

	newBytes, err := io.ReadAll(f)
	if err != nil || len(newBytes) == 0 {
		return
	}

	// Process complete lines only (keep partial trailing line for next read)
	consumed := 0
	for _, line := range splitCompleteLines(newBytes) {
		consumed += len(line) + 1 // +1 for newline
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
// A trailing partial line is excluded — it'll be picked up on the next read.
func splitCompleteLines(data []byte) [][]byte {
	var lines [][]byte
	for len(data) > 0 {
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break // partial line — stop
		}
		lines = append(lines, data[:idx])
		data = data[idx+1:]
	}
	return lines
}
```

Add `"bytes"` to imports.

**Step 2: Build**

Run: `go build ./...`
Expected: builds clean

**Step 3: Commit**

```bash
git add internal/watcher/watcher.go
git commit -m "fix(watcher): track byte offset correctly with partial line handling"
```

---

### Task 7: Update docs and clean up

**Files:**
- Modify: `docs/BRIDGE_TIMING.md` — delete this file (no longer relevant, the race condition is eliminated)
- Modify: `CLAUDE.md` — update architecture section to mention watcher instead of bridge

**Step 1: Delete bridge timing doc**

Delete `docs/BRIDGE_TIMING.md`.

**Step 2: Update CLAUDE.md**

In the project structure section, replace:

```
  ├── bridge/   - CC Stop hook bridge (JSONL → daemon → Telegram)
```

with:

```
  ├── watcher/  - JSONL file watcher (CC → Telegram via daemon)
```

**Step 3: Commit**

```bash
git add -A
git commit -m "docs: update architecture for daemon JSONL watcher"
```

---

### Task 8: Build, install, test end-to-end

**Step 1: Build and install**

Run: `make install`

**Step 2: Restart daemon**

Run: `make reinstall` (or `launchctl kickstart -k "gui/$(id -u)/io.guion.ttal.daemon"`)

**Step 3: Test**

1. Open a CC session for an agent (e.g. athena)
2. Ask it to say something
3. Check Telegram — the text should appear as it's written
4. Verify multiple text blocks in one turn each arrive as separate messages

**Step 4: Verify no Stop hook**

Check `~/.claude/settings.json` — the Stop hook entry should be gone.

---

## Key Design Decisions

1. **fsnotify per-directory, not recursive**: We watch specific agent project dirs, not all of `~/.claude/projects/`. This avoids noise from non-agent sessions.

2. **New file → seek to end**: When the watcher first sees a JSONL file, it skips to the end. This prevents replaying old conversations on daemon restart. Tradeoff: the first message after daemon restart may be missed. Acceptable for v1.

3. **Offsets in memory only**: Lost on daemon restart. Acceptable because the daemon is a long-running launchd service. On restart, new files start from end (see above).

4. **No turn detection**: Each assistant text block is sent independently. No need to detect turn boundaries, no staleness markers, no retry loops.

5. **Daemon opens DB**: The daemon now opens `~/.ttal/ttal.db` read-only for agent queries. This is a new dependency but minimal — only used at startup to build the agent map.

6. **Graceful degradation**: If the watcher fails to initialize (e.g. no agents, no DB), the daemon logs a warning and continues running all other services normally.

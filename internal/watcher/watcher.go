package watcher

import (
	"bytes"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// ToolFunc is called when a tool invocation is detected in CC JSONL.
type ToolFunc func(teamName, agentName, toolName string)

// WatchedAgent pairs agent info with the projects dir and encoded dir name for its JSONL.
// Exported because daemon.go constructs the map.
type WatchedAgent struct {
	AgentInfo
	ProjectsDir string // which projects/ dir this agent's JSONL lives in
	EncodedDir  string // the CC-encoded dir name (e.g. "-workspace-manager")
}

// Watcher tails active CC JSONL files and sends assistant text to Telegram.
type Watcher struct {
	agents   map[string]WatchedAgent // composite key "team/encoded" -> agent
	dirToKey map[string]string       // full dir path -> composite key (for fsnotify lookup)
	offsets  map[string]int64        // file path -> last read offset
	mu       sync.Mutex
	send     SendFunc
	onTool   ToolFunc
}

// EncodePath converts an absolute path to CC's encoded project directory name.
// CC replaces / and . with - (e.g. /Users/neil/clawd -> -Users-neil-clawd).
func EncodePath(path string) string {
	encoded := strings.ReplaceAll(path, string(filepath.Separator), "-")
	encoded = strings.ReplaceAll(encoded, ".", "-")
	return encoded
}

// New creates a Watcher from a pre-built agent map.
// Key is composite "teamName/encodedDir" to avoid collisions across teams.
// Config-driven: no DB or config.Load() required.
func New(agents map[string]WatchedAgent, send SendFunc, onTool ToolFunc) (*Watcher, error) {
	log.Printf("[watcher] watching %d agents", len(agents))

	dirToKey := make(map[string]string, len(agents))
	for key, agent := range agents {
		fullDir := filepath.Join(agent.ProjectsDir, agent.EncodedDir)
		dirToKey[fullDir] = key
	}

	return &Watcher{
		agents:   agents,
		dirToKey: dirToKey,
		offsets:  make(map[string]int64),
		send:     send,
		onTool:   onTool,
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
	// Dirs are created by ensureProjectDir before agent spawn — skip any that
	// don't exist yet (agent not spawned or dir cleaned up).
	for _, agent := range w.agents {
		dir := filepath.Join(agent.ProjectsDir, agent.EncodedDir)
		if err := fsw.Add(dir); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("[watcher] failed to watch %s: %v", agent.AgentName, err)
			}
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
	dirPath := filepath.Dir(path)
	key, ok := w.dirToKey[dirPath]
	if !ok {
		return
	}
	agentInfo := w.agents[key].AgentInfo

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

	consumed := w.processLines(newBytes, agentInfo)

	w.mu.Lock()
	w.offsets[path] = offset + int64(consumed)
	w.mu.Unlock()
}

// processLines processes complete JSONL lines, dispatching tools and text events.
// Returns the number of bytes consumed (including newlines).
func (w *Watcher) processLines(data []byte, agent AgentInfo) int {
	consumed := 0
	for _, line := range splitCompleteLines(data) {
		consumed += len(line) + 1

		// Tool-use path (CC native tools, not <cmd> blocks).
		if toolName := extractToolUse(line); toolName != "" {
			if w.onTool != nil {
				w.onTool(agent.TeamName, agent.AgentName, toolName)
			}
		}

		// Telegram prose — forward raw assistant text without cmd stripping.
		if prose := extractAssistantText(line); prose != "" && !isNoisyText(prose) {
			w.send(agent.TeamName, agent.AgentName, prose)
		}
	}
	return consumed
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

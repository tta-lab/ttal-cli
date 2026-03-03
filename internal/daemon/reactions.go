package daemon

import "sync"

// trackedMessage holds the Telegram message ID for the most recent
// inbound message to an agent, so reactions can be set on it.
type trackedMessage struct {
	ChatID    int64
	MessageID int
	BotToken  string
}

// messageTracker stores the most recent inbound Telegram message per agent.
// Key: "team:agent"
type messageTracker struct {
	mu    sync.RWMutex
	store map[string]trackedMessage
}

func newMessageTracker() *messageTracker {
	return &messageTracker{store: make(map[string]trackedMessage)}
}

func trackerKey(teamName, agentName string) string {
	return teamName + ":" + agentName
}

func (mt *messageTracker) set(teamName, agentName string, msg trackedMessage) {
	mt.mu.Lock()
	mt.store[trackerKey(teamName, agentName)] = msg
	mt.mu.Unlock()
}

func (mt *messageTracker) get(teamName, agentName string) (trackedMessage, bool) {
	mt.mu.RLock()
	msg, ok := mt.store[trackerKey(teamName, agentName)]
	mt.mu.RUnlock()
	return msg, ok
}

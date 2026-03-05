package tui

import tea "charm.land/bubbletea/v2"

type keyAction int

const (
	keyNone keyAction = iota
	keyQuit
	keyUp
	keyDown
	keyEnter
	keyEsc
	keyExecute
	keyRoute
	keyOpenPR
	keyOpenSession
	keyOpenTerm
	keyOpenEditor
	keyAddToday
	keyRemoveToday
	keyFilter
	keySearch
	keyHelp
	keyRefresh
	keyPageUp
	keyPageDown
	keyHalfPageUp
	keyHalfPageDown
	keyTop
	keyBottom
	keyDone
	keyModify
	keyAnnotate
	keyToggleNext
)

var keyMap = map[string]keyAction{
	"q":      keyQuit,
	"ctrl+c": keyQuit,
	"k":      keyUp,
	"up":     keyUp,
	"j":      keyDown,
	"down":   keyDown,
	"enter":  keyEnter,
	"esc":    keyEsc,
	"x":      keyExecute,
	"r":      keyRoute,
	"p":      keyOpenPR,
	"s":      keyOpenSession,
	"o":      keyOpenTerm,
	"t":      keyToggleNext,
	"e":      keyOpenEditor,
	"a":      keyAddToday,
	"ctrl+a": keyRemoveToday,
	"d":      keyDone,
	"m":      keyModify,
	"A":      keyAnnotate,
	"f":      keyFilter,
	"/":      keySearch,
	"?":      keyHelp,
	"ctrl+r": keyRefresh,
	"ctrl+f": keyPageDown,
	"pgdown": keyPageDown,
	"ctrl+b": keyPageUp,
	"pgup":   keyPageUp,
	"ctrl+d": keyHalfPageDown,
	"ctrl+u": keyHalfPageUp,
	"g":      keyTop,
	"G":      keyBottom,
}

func resolveKey(msg tea.KeyPressMsg) keyAction {
	if action, ok := keyMap[msg.String()]; ok {
		return action
	}
	return keyNone
}

const helpText = `Key Bindings:

  j/k, Up/Down   Navigate tasks
  Enter           Task detail
  Esc             Back / close overlay
  g/G             Top / bottom
  Ctrl+D/U        Half page down/up

  x               Execute (spawn worker)
  r               Route to agent
  d               Mark task done
  m               Modify task
  A               Annotate task
  p               Open PR in browser
  s               Attach tmux session
  o               Open terminal
  t               Toggle +next tag
  e               Open editor

  a               Add to today
  Ctrl+A          Remove from today
  f               Cycle filter (pending/today/active/completed)
  /               Search (taskwarrior syntax: project:x +tag etc)
  Ctrl+R          Refresh tasks

  ?               Toggle help
  q               Quit`

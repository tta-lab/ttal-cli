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
	"o":      keyOpenPR,
	"s":      keyOpenSession,
	"t":      keyOpenTerm,
	"e":      keyOpenEditor,
	"a":      keyAddToday,
	"A":      keyRemoveToday,
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
  o               Open PR in browser
  s               Attach tmux session
  t               Open terminal
  e               Open editor

  a               Add to today
  A               Remove from today
  f               Cycle filter
  /               Search
  Ctrl+R          Refresh tasks

  ?               Toggle help
  q               Quit`

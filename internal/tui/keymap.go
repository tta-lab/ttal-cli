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

func resolveKey(msg tea.KeyPressMsg) keyAction {
	s := msg.String()
	switch s {
	case "q", "ctrl+c":
		return keyQuit
	case "k", "up":
		return keyUp
	case "j", "down":
		return keyDown
	case "enter":
		return keyEnter
	case "esc":
		return keyEsc
	case "x":
		return keyExecute
	case "r":
		return keyRoute
	case "o":
		return keyOpenPR
	case "s":
		return keyOpenSession
	case "t":
		return keyOpenTerm
	case "e":
		return keyOpenEditor
	case "a":
		return keyAddToday
	case "A":
		return keyRemoveToday
	case "f":
		return keyFilter
	case "/":
		return keySearch
	case "?":
		return keyHelp
	case "ctrl+r":
		return keyRefresh
	case "ctrl+f", "pgdown":
		return keyPageDown
	case "ctrl+b", "pgup":
		return keyPageUp
	case "ctrl+d":
		return keyHalfPageDown
	case "ctrl+u":
		return keyHalfPageUp
	case "g":
		return keyTop
	case "G":
		return keyBottom
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

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
	keyAdvance
	keyOpenPR
	keyOpenSession
	keyOpenTerm
	keyOpenEditor
	keyAddToday
	keyRemoveToday
	keyFilterNext
	keyFilterPrev
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
	keyCopy
	keyDelete
	keyHeatmap
	keyLeft
	keyRight
)

var keyMap = map[string]keyAction{
	"q":      keyQuit,
	"ctrl+c": keyQuit,
	"k":      keyUp,
	"up":     keyUp,
	"h":      keyLeft,
	"left":   keyLeft,
	"l":      keyRight,
	"right":  keyRight,
	"j":      keyDown,
	"down":   keyDown,
	"enter":  keyEnter,
	"esc":    keyEsc,
	"g":      keyAdvance,
	"p":      keyOpenPR,
	"s":      keyOpenSession,
	"o":      keyOpenTerm,
	"t":      keyToggleNext,
	"y":      keyCopy,
	"e":      keyOpenEditor,
	"a":      keyAddToday,
	"ctrl+a": keyRemoveToday,
	"d":      keyDone,
	"D":      keyDelete,
	"m":      keyModify,
	"A":      keyAnnotate,
	"[":      keyFilterPrev,
	"]":      keyFilterNext,
	"/":      keySearch,
	"H":      keyHeatmap,
	"?":      keyHelp,
	"ctrl+r": keyRefresh,
	"ctrl+f": keyPageDown,
	"pgdown": keyPageDown,
	"ctrl+b": keyPageUp,
	"pgup":   keyPageUp,
	"ctrl+d": keyHalfPageDown,
	"ctrl+u": keyHalfPageUp,
	"home":   keyTop,
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
  Home            Top
  G               Bottom
  Ctrl+D/U        Half page down/up

  g               Advance task (go — pipeline stage)
  d               Mark task done
  m               Modify task
  A               Annotate task
  p               Open PR in browser
  s               Attach tmux session
  o               Open terminal
  t               Toggle +next tag
  y               Copy task to clipboard
  e               Edit task (task edit)
  D               Delete task (with confirmation)

  a               Add to today
  Ctrl+A          Remove from today
  [               Previous filter
  ]               Next filter
  /               Search (taskwarrior syntax: project:x +tag etc)
  Ctrl+W          Delete word (in search)
  Ctrl+C          Cancel search
  Ctrl+R          Refresh tasks

  H               Heatmap (task completion, past year) — hjkl/←↑↓→ to navigate, H/Esc to close
  ?               Toggle help
  q               Quit`

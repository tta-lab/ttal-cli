package tui

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

type viewState int

const (
	stateTaskList viewState = iota
	stateTaskDetail
	stateRouteInput
	stateSearch
	stateHelp
)

// Task extends taskwarrior.Task with display fields parsed during export.
type Task struct {
	taskwarrior.Task
	Priority  string  `json:"priority,omitempty"`
	Urgency   float64 `json:"urgency"`
	Scheduled string  `json:"scheduled,omitempty"`
	Due       string  `json:"due,omitempty"`
}

func (t *Task) ShortUUID() string {
	if len(t.UUID) >= 8 {
		return t.UUID[:8]
	}
	return t.UUID
}

// IsToday returns true if the task is scheduled for today or earlier.
func (t *Task) IsToday() bool {
	if t.Scheduled == "" {
		return false
	}
	parsed, err := parseTaskDate(t.Scheduled)
	if err != nil {
		return false
	}
	today := time.Now().Truncate(24 * time.Hour)
	return !parsed.After(today)
}

func parseTaskDate(s string) (time.Time, error) {
	formats := []string{
		"20060102T150405Z",
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.Truncate(24 * time.Hour), nil
		}
	}
	return time.Time{}, fmt.Errorf("unparseable date: %s", s)
}

type filterMode int

const (
	filterPending filterMode = iota
	filterToday
	filterActive
	filterCompleted
)

func (f filterMode) String() string {
	switch f {
	case filterPending:
		return "pending"
	case filterToday:
		return "today"
	case filterActive:
		return "active"
	case filterCompleted:
		return "completed"
	default:
		return "pending"
	}
}

func (f filterMode) Next() filterMode {
	return (f + 1) % 4
}

type Model struct {
	state  viewState
	filter filterMode

	// Data
	tasks    []Task
	filtered []Task
	agents   []agentfs.AgentInfo
	cfg      *config.Config

	// Task list
	cursor    int
	offset    int // viewport scroll offset
	searchStr string

	// Route input
	routeInput   string
	routeMatches []agentfs.AgentInfo

	// Layout
	width  int
	height int

	// Status
	statusMsg string
	err       error
	loading   bool
	showHelp  bool
}

// NewModel creates the initial TUI model.
func NewModel() Model {
	return Model{
		state:   stateTaskList,
		filter:  filterPending,
		loading: true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfig(), loadTasks(filterPending, ""))
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case configLoadedMsg:
		if msg.err != nil {
			m.statusMsg = "Config error: " + msg.err.Error()
		}
		m.cfg = msg.cfg
		m.agents = msg.agents
		return m, nil

	case tasksLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.statusMsg = "Task load error: " + msg.err.Error()
		}
		m.tasks = msg.tasks
		m.applyFilter()
		return m, nil

	case actionResultMsg:
		m.statusMsg = msg.message
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
		}
		if msg.refresh {
			return m, loadTasks(m.filter, m.searchStr)
		}
		return m, nil

	case execFinishedMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) View() tea.View {
	if m.width == 0 {
		v := tea.NewView("Loading...")
		v.AltScreen = true
		return v
	}

	var content string
	switch m.state {
	case stateTaskList, stateSearch:
		content = m.viewTaskList()
	case stateTaskDetail:
		content = m.viewTaskDetail()
	case stateRouteInput:
		content = m.viewTaskList() // show list behind overlay
	case stateHelp:
		content = m.viewHelp()
	}

	// Overlay for route input
	if m.state == stateRouteInput {
		content = m.viewRouteOverlay(content)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Route input mode handles keys differently
	if m.state == stateRouteInput {
		return m.handleRouteKey(msg)
	}

	// Search mode handles keys differently
	if m.state == stateSearch {
		return m.handleSearchKey(msg)
	}

	action := resolveKey(msg)

	switch action {
	case keyQuit:
		if m.state != stateTaskList {
			m.state = stateTaskList
			return m, nil
		}
		return m, tea.Quit

	case keyUp:
		m.moveCursor(-1)
	case keyDown:
		m.moveCursor(1)
	case keyPageDown:
		m.moveCursor(m.visibleRows())
	case keyPageUp:
		m.moveCursor(-m.visibleRows())
	case keyHalfPageDown:
		m.moveCursor(m.visibleRows() / 2)
	case keyHalfPageUp:
		m.moveCursor(-m.visibleRows() / 2)
	case keyTop:
		m.cursor = 0
		m.offset = 0
	case keyBottom:
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.ensureCursorVisible()
		}

	case keyEnter:
		if len(m.filtered) > 0 {
			m.state = stateTaskDetail
		}
	case keyEsc:
		if m.state != stateTaskList {
			m.state = stateTaskList
		}

	case keyExecute:
		if t := m.selectedTask(); t != nil {
			return m, executeTask(t.UUID)
		}
	case keyRoute:
		if len(m.filtered) > 0 {
			m.state = stateRouteInput
			m.routeInput = ""
			m.updateRouteMatches()
		}
	case keyOpenPR:
		if t := m.selectedTask(); t != nil {
			return m, openPR(t.UUID)
		}
	case keyOpenSession:
		if t := m.selectedTask(); t != nil {
			return m, openSession(t)
		}
	case keyOpenTerm:
		if t := m.selectedTask(); t != nil {
			return m, openTerm(t)
		}
	case keyOpenEditor:
		if t := m.selectedTask(); t != nil {
			return m, openEditor(t)
		}
	case keyAddToday:
		if t := m.selectedTask(); t != nil {
			return m, addToToday(t.UUID)
		}
	case keyRemoveToday:
		if t := m.selectedTask(); t != nil {
			return m, removeFromToday(t.UUID)
		}
	case keyFilter:
		m.filter = m.filter.Next()
		m.cursor = 0
		m.offset = 0
		m.loading = true
		return m, loadTasks(m.filter, m.searchStr)
	case keySearch:
		m.state = stateSearch
		m.searchStr = ""
	case keyHelp:
		if m.state == stateHelp {
			m.state = stateTaskList
		} else {
			m.state = stateHelp
		}
	case keyRefresh:
		m.loading = true
		return m, loadTasks(m.filter, m.searchStr)
	}

	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "enter":
		m.state = stateTaskList
		m.cursor = 0
		m.offset = 0
		m.loading = true
		return m, loadTasks(m.filter, m.searchStr)
	case "esc":
		m.state = stateTaskList
		m.searchStr = ""
		m.cursor = 0
		m.offset = 0
		m.loading = true
		return m, loadTasks(m.filter, "")
	case "backspace":
		if len(m.searchStr) > 0 {
			m.searchStr = m.searchStr[:len(m.searchStr)-1]
		}
	default:
		if len(msg.Text) > 0 {
			m.searchStr += msg.Text
		}
	}
	return m, nil
}

func (m *Model) handleRouteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case "enter":
		if len(m.routeMatches) > 0 {
			agent := m.routeMatches[0]
			t := m.selectedTask()
			if t != nil && agent.Role != "" {
				m.state = stateTaskList
				return m, routeTask(t.UUID, agent.Name)
			}
			if agent.Role == "" {
				m.statusMsg = "Agent " + agent.Name + " has no role"
			}
		}
		m.state = stateTaskList
	case "esc":
		m.state = stateTaskList
	case "tab":
		if len(m.routeMatches) > 0 {
			m.routeInput = m.routeMatches[0].Name
			m.updateRouteMatches()
		}
	case "backspace":
		if len(m.routeInput) > 0 {
			m.routeInput = m.routeInput[:len(m.routeInput)-1]
			m.updateRouteMatches()
		}
	default:
		if len(msg.Text) > 0 {
			m.routeInput += msg.Text
			m.updateRouteMatches()
		}
	}
	return m, nil
}

func (m *Model) updateRouteMatches() {
	if m.routeInput == "" {
		// Show all agents with a role
		m.routeMatches = nil
		for _, a := range m.agents {
			if a.Role != "" {
				m.routeMatches = append(m.routeMatches, a)
			}
		}
		return
	}
	q := strings.ToLower(m.routeInput)
	m.routeMatches = nil
	for _, a := range m.agents {
		if a.Role != "" && strings.Contains(strings.ToLower(a.Name), q) {
			m.routeMatches = append(m.routeMatches, a)
		}
	}
}

func (m *Model) selectedTask() *Task {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		return &m.filtered[m.cursor]
	}
	return nil
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	visible := m.visibleRows()
	if visible <= 0 {
		return
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m *Model) visibleRows() int {
	// Header (1) + title (1) + status bar (1) + border space (1) = 4 lines overhead
	rows := m.height - 4
	if rows < 1 {
		rows = 1
	}
	return rows
}

func (m *Model) applyFilter() {
	m.filtered = nil
	for _, t := range m.tasks {
		switch m.filter {
		case filterToday:
			if !t.IsToday() {
				continue
			}
		case filterActive:
			if t.Start == "" {
				continue
			}
		}
		m.filtered = append(m.filtered, t)
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// Messages

type configLoadedMsg struct {
	cfg    *config.Config
	agents []agentfs.AgentInfo
	err    error
}

type tasksLoadedMsg struct {
	tasks []Task
	err   error
}

type actionResultMsg struct {
	message string
	err     error
	refresh bool
}

type execFinishedMsg struct {
	err error
}

// Commands

func loadConfig() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return configLoadedMsg{err: fmt.Errorf("load config: %w", err)}
		}
		agents, _ := agentfs.Discover(cfg.TeamPath())
		return configLoadedMsg{cfg: cfg, agents: agents}
	}
}

func loadTasks(filter filterMode, search string) tea.Cmd {
	return func() tea.Msg {
		var args []string
		switch filter {
		case filterPending, filterToday, filterActive:
			args = append(args, "status:pending")
		case filterCompleted:
			args = append(args, "status:completed")
		}

		if search != "" {
			args = append(args, "description.contains:"+search)
		}

		args = append(args, "export")

		cmd := taskwarrior.Command(args...)
		out, err := cmd.Output()
		if err != nil {
			return tasksLoadedMsg{err: fmt.Errorf("taskwarrior: %w", err)}
		}

		var tasks []Task
		if err := json.Unmarshal(out, &tasks); err != nil {
			return tasksLoadedMsg{err: fmt.Errorf("parse tasks: %w", err)}
		}

		return tasksLoadedMsg{tasks: tasks}
	}
}

package tui

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// Key name constants for input handlers.
const (
	keyNameEnter     = "enter"
	keyNameEsc       = "esc"
	keyNameBackspace = "backspace"
	keyNameTab       = "tab"
)

type viewState int

const (
	stateTaskList viewState = iota
	stateTaskDetail
	stateRouteInput
	stateSearch
	stateModify
	stateAnnotate
	stateHelp
)

// Task extends taskwarrior.Task with display fields parsed during export.
type Task struct {
	taskwarrior.Task
	Priority  string  `json:"priority,omitempty"`
	Urgency   float64 `json:"urgency"`
	Scheduled string  `json:"scheduled,omitempty"`
	Due       string  `json:"due,omitempty"`
	Entry     string  `json:"entry,omitempty"`
}

func (t *Task) ShortUUID() string {
	if len(t.UUID) >= 8 {
		return t.UUID[:8]
	}
	return t.UUID
}

func (t *Task) Age() string {
	if t.Entry == "" {
		return ""
	}
	parsed, err := time.Parse("20060102T150405Z", t.Entry)
	if err != nil {
		return "?"
	}
	return formatAge(time.Since(parsed))
}

func formatAge(d time.Duration) string {
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(math.Round(d.Hours())))
	}
	if d < 30*24*time.Hour {
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
	return fmt.Sprintf("%dmo", int(d.Hours()/24/30))
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

type modifyMatch struct {
	Type  string
	Value string
}

const (
	modifierProject  = "project:"
	modifierTag      = "+"
	modifierPriority = "priority:"
	modifierStatus   = "status:"
)

const (
	matchTypeProject  = "project"
	matchTypeTag      = "tag"
	matchTypePriority = "priority"
	matchTypeStatus   = "status"
)

var priorityValues = []string{"H", "M", "L"}
var statusValues = []string{"pending", "completed", "waiting", "deleted"}

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

func (f filterMode) Prev() filterMode {
	return (f - 1 + 4) % 4
}

type Model struct {
	state  viewState
	filter filterMode

	// Data
	tasks    []Task
	filtered []Task
	agents   []agentfs.AgentInfo
	cfg      *config.Config
	projects []string
	tags     []string

	// Task list cursor
	cursor    int
	offset    int
	searchStr string

	// Route input
	routeInput   string
	routeMatches []agentfs.AgentInfo

	// Text input for overlays
	modifyInput   string
	annotateInput string

	// Modify input autocomplete
	modifyMatches []modifyMatch

	// Layout
	width  int
	height int

	// Status
	statusMsg string
	loading   bool
	teamName  string
}

func NewModel() Model {
	m := Model{
		state:   stateTaskList,
		filter:  filterPending,
		loading: true,
		offset:  0,
	}
	return m
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
		m.projects = msg.projects
		m.tags = msg.tags
		if msg.cfg != nil {
			m.teamName = msg.cfg.TeamName()
		}
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
			return m, m.reloadTasks()
		}
		return m, nil

	case execFinishedMsg:
		if msg.err != nil {
			m.statusMsg = "Error: " + msg.err.Error()
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.PasteMsg:
		return m.handlePaste(msg)
	}
	return m, nil
}

func (m *Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	text := msg.Content
	switch m.state {
	case stateSearch:
		m.searchStr += text
	case stateModify:
		m.modifyInput += text
	case stateAnnotate:
		m.annotateInput += text
	case stateRouteInput:
		m.routeInput += text
		m.updateRouteMatches()
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
	case stateRouteInput, stateModify, stateAnnotate:
		content = m.viewTaskList() // show list behind overlay
	case stateHelp:
		content = m.viewHelp()
	}

	// Overlays
	switch m.state {
	case stateRouteInput:
		content = m.viewRouteOverlay(content)
	case stateModify:
		content = m.viewModifyOverlay(content)
	case stateAnnotate:
		content = m.viewTextInputOverlay(content, "Annotate Task", "Annotation:", m.annotateInput)
	}

	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

func (m *Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch m.state {
	case stateRouteInput:
		return m.handleRouteKey(msg)
	case stateSearch:
		return m.handleSearchKey(msg)
	case stateModify:
		return m.handleModifyKey(msg)
	case stateAnnotate:
		return m.handleAnnotateKey(msg)
	}

	action := resolveKey(msg)

	switch action {
	case keyQuit:
		if m.state != stateTaskList {
			m.state = stateTaskList
			return m, nil
		}
		return m, tea.Quit
	case keyEsc:
		if m.state != stateTaskList {
			m.state = stateTaskList
		}
		return m, nil
	}

	if m.handleNavigation(action) {
		return m, nil
	}

	return m.handleAction(action)
}

func (m *Model) handleNavigation(action keyAction) bool {
	switch action {
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
		m.ensureCursorVisible()
	case keyBottom:
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.ensureCursorVisible()
		}
	case keyEnter:
		if len(m.filtered) > 0 {
			m.state = stateTaskDetail
		}
	default:
		return false
	}
	return true
}

func (m *Model) handleAction(action keyAction) (tea.Model, tea.Cmd) {
	// Task-scoped actions that require a selected task
	if cmd := m.handleTaskAction(action); cmd != nil {
		return m, cmd
	}

	// Global actions
	switch action {
	case keyRoute:
		if len(m.filtered) > 0 {
			m.state = stateRouteInput
			m.routeInput = ""
			m.updateRouteMatches()
		}
	case keyFilterNext:
		m.filter = m.filter.Next()
		m.cursor = 0
		return m, m.reloadTasks()
	case keyFilterPrev:
		m.filter = m.filter.Prev()
		m.cursor = 0
		return m, m.reloadTasks()
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
		return m, m.reloadTasks()
	}
	return m, nil
}

func (m *Model) handleTaskAction(action keyAction) tea.Cmd {
	t := m.selectedTask()
	if t == nil {
		return nil
	}
	switch action {
	case keyExecute:
		return executeTask(t.UUID)
	case keyOpenPR:
		return openPR(t.UUID)
	case keyOpenSession:
		return openSession(t)
	case keyOpenTerm:
		return openTerm(t)
	case keyOpenEditor:
		return openEditor(t)
	case keyAddToday:
		return addToToday(t.UUID)
	case keyRemoveToday:
		return removeFromToday(t.UUID)
	case keyToggleNext:
		return toggleNext(t)
	case keyDone:
		return doneTask(t.UUID)
	case keyCopy:
		return copyTask(t)
	case keyModify:
		m.state = stateModify
		m.modifyInput = ""
		m.updateModifyMatches(m.projects, m.tags)
		return nil
	case keyAnnotate:
		m.state = stateAnnotate
		m.annotateInput = ""
		return nil
	}
	return nil
}

func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		m.state = stateTaskList
		m.cursor = 0
		return m, m.reloadTasks()
	case keyNameEsc:
		m.state = stateTaskList
		m.searchStr = ""
		m.cursor = 0
		return m, m.reloadTasks()
	case keyNameBackspace:
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

func (m *Model) handleModifyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		t := m.selectedTask()
		if t != nil && m.modifyInput != "" {
			m.state = stateTaskList
			return m, modifyTask(t.UUID, m.modifyInput)
		}
		m.state = stateTaskList
	case keyNameEsc:
		m.state = stateTaskList
	case keyNameTab:
		if len(m.modifyMatches) > 0 {
			match := m.modifyMatches[0]
			switch match.Type {
			case matchTypeProject:
				m.modifyInput = modifierProject + match.Value + " "
			case matchTypeTag:
				m.modifyInput = modifierTag + match.Value + " "
			case matchTypePriority:
				m.modifyInput = modifierPriority + match.Value + " "
			case matchTypeStatus:
				m.modifyInput = modifierStatus + match.Value + " "
			}
			m.updateModifyMatches(m.projects, m.tags)
		}
	case keyNameBackspace:
		if len(m.modifyInput) > 0 {
			m.modifyInput = m.modifyInput[:len(m.modifyInput)-1]
		}
		m.updateModifyMatches(m.projects, m.tags)
	default:
		if len(msg.Text) > 0 {
			m.modifyInput += msg.Text
			m.updateModifyMatches(m.projects, m.tags)
		}
	}
	return m, nil
}

func (m *Model) handleAnnotateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		t := m.selectedTask()
		if t != nil && m.annotateInput != "" {
			m.state = stateTaskList
			return m, annotateTask(t.UUID, m.annotateInput)
		}
		m.state = stateTaskList
	case keyNameEsc:
		m.state = stateTaskList
	case keyNameBackspace:
		if len(m.annotateInput) > 0 {
			m.annotateInput = m.annotateInput[:len(m.annotateInput)-1]
		}
	default:
		if len(msg.Text) > 0 {
			m.annotateInput += msg.Text
		}
	}
	return m, nil
}

func (m *Model) handleRouteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
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
	case keyNameEsc:
		m.state = stateTaskList
	case keyNameTab:
		if len(m.routeMatches) > 0 {
			m.routeInput = m.routeMatches[0].Name
			m.updateRouteMatches()
		}
	case keyNameBackspace:
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
	q := strings.ToLower(m.routeInput)
	m.routeMatches = nil
	for _, a := range m.agents {
		if q == "" || strings.Contains(strings.ToLower(a.Name), q) {
			m.routeMatches = append(m.routeMatches, a)
		}
	}
}

func (m *Model) updateModifyMatches(projects, tags []string) {
	m.modifyMatches = nil
	input := m.modifyInput

	switch {
	case strings.HasPrefix(input, modifierProject):
		m.updateProjectMatches(projects, strings.TrimPrefix(input, modifierProject))
	case strings.HasPrefix(input, modifierTag):
		m.updateTagMatches(tags, strings.TrimPrefix(input, modifierTag))
	case strings.HasPrefix(input, modifierPriority):
		m.updatePriorityMatches(strings.TrimPrefix(input, modifierPriority))
	case strings.HasPrefix(input, modifierStatus):
		m.updateStatusMatches(strings.TrimPrefix(input, modifierStatus))
	case input == "":
		m.updateAllMatches(projects, tags)
	}
}

func (m *Model) updateProjectMatches(projects []string, query string) {
	q := strings.ToLower(query)
	for _, p := range projects {
		if q == "" || strings.Contains(strings.ToLower(p), q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeProject, Value: p})
		}
	}
}

func (m *Model) updateTagMatches(tags []string, query string) {
	q := strings.ToLower(query)
	for _, t := range tags {
		if q == "" || strings.Contains(strings.ToLower(t), q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeTag, Value: t})
		}
	}
}

func (m *Model) updatePriorityMatches(query string) {
	q := strings.ToLower(query)
	for _, p := range priorityValues {
		if q == "" || strings.Contains(p, q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypePriority, Value: p})
		}
	}
}

func (m *Model) updateStatusMatches(query string) {
	q := strings.ToLower(query)
	for _, s := range statusValues {
		if q == "" || strings.Contains(s, q) {
			m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeStatus, Value: s})
		}
	}
}

func (m *Model) updateAllMatches(projects, tags []string) {
	for _, p := range projects {
		m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeProject, Value: p})
	}
	for _, t := range tags {
		m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypeTag, Value: t})
	}
	for _, p := range priorityValues {
		m.modifyMatches = append(m.modifyMatches, modifyMatch{Type: matchTypePriority, Value: p})
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
	if len(m.filtered) > 0 && m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	m.ensureCursorVisible()
}

func (m *Model) ensureCursorVisible() {
	if m.offset > m.cursor {
		m.offset = m.cursor
	}
	visible := m.visibleRows()
	if m.offset+visible > len(m.filtered) {
		m.offset = len(m.filtered) - visible
	}
	if m.offset < 0 {
		m.offset = 0
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
	sort.Slice(m.filtered, func(i, j int) bool {
		return m.filtered[i].Urgency > m.filtered[j].Urgency
	})
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.offset = 0
}

// Messages

type configLoadedMsg struct {
	cfg      *config.Config
	agents   []agentfs.AgentInfo
	projects []string
	tags     []string
	err      error
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

		projects, err := taskwarrior.GetProjects()
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := taskwarrior.GetTags()
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return configLoadedMsg{cfg: cfg, agents: agents, projects: projects, tags: tags}
	}
}

func (m *Model) reloadTasks() tea.Cmd {
	m.loading = true
	return loadTasks(m.filter, m.searchStr)
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

		// Pass search as raw taskwarrior filter args
		if search != "" {
			args = append(args, strings.Fields(search)...)
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

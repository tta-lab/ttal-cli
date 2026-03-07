package tui

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

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
	keyNameCtrlC     = "ctrl+c"
	keyNameCtrlN     = "ctrl+n"
	keyNameCtrlP     = "ctrl+p"
	keyNameCtrlW     = "ctrl+w"
)

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
	modifyIndex   int

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

	case autocompleteLoadedMsg:
		m.projects = msg.projects
		m.tags = msg.tags
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
	case stateSearch:
		content = m.viewSearchOverlay(content)
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
		if len(m.projects) == 0 || len(m.tags) == 0 {
			return m, loadConfigForAutocomplete()
		}
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
		m.modifyIndex = 0
		return m, m.reloadTasks()
	case keyNameEsc, keyNameCtrlC:
		m.state = stateTaskList
		m.searchStr = ""
		m.cursor = 0
		m.modifyIndex = 0
		return m, m.reloadTasks()
	case keyNameBackspace:
		m.handleSearchBackspace()
	case keyNameTab:
		m.handleSearchTab()
	case keyNameCtrlN, keyNameCtrlP:
		m.navigateSearchMatches(s == keyNameCtrlN)
	case keyNameCtrlW:
		m.handleSearchCtrlW()
	default:
		m.handleSearchInput(msg)
	}
	return m, nil
}

func (m *Model) handleSearchBackspace() {
	if len(m.searchStr) > 0 {
		m.searchStr = m.searchStr[:len(m.searchStr)-1]
	}
	m.updateSearchMatches(m.projects, m.tags)
}

func (m *Model) handleSearchTab() {
	if len(m.modifyMatches) == 0 {
		return
	}
	m.modifyIndex = (m.modifyIndex + 1) % len(m.modifyMatches)
	match := m.modifyMatches[m.modifyIndex]
	switch match.Type {
	case matchTypeProject:
		m.searchStr = "project:" + match.Value + " "
	case matchTypeTag:
		m.searchStr = "+" + match.Value + " "
	default:
		m.searchStr += match.Value + " "
	}
	m.updateSearchMatches(m.projects, m.tags)
}

func (m *Model) navigateSearchMatches(next bool) {
	if len(m.modifyMatches) == 0 {
		return
	}
	if next {
		m.modifyIndex = (m.modifyIndex + 1) % len(m.modifyMatches)
	} else {
		m.modifyIndex = (m.modifyIndex - 1 + len(m.modifyMatches)) % len(m.modifyMatches)
	}
}

func (m *Model) handleSearchCtrlW() {
	if len(m.searchStr) > 0 {
		m.searchStr = deleteLastWord(m.searchStr)
		m.updateSearchMatches(m.projects, m.tags)
	}
}

func (m *Model) handleSearchInput(msg tea.KeyPressMsg) {
	if len(msg.Text) > 0 {
		m.searchStr += msg.Text
		m.modifyIndex = 0
		m.updateSearchMatches(m.projects, m.tags)
	}
}

func (m *Model) handleModifyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		t := m.selectedTask()
		if t != nil && m.modifyInput != "" {
			m.state = stateTaskList
			m.modifyIndex = 0
			return m, modifyTask(t.UUID, m.modifyInput)
		}
		m.state = stateTaskList
		m.modifyIndex = 0
	case keyNameEsc:
		m.state = stateTaskList
		m.modifyIndex = 0
	case keyNameTab:
		m.handleModifyTab()
	case keyNameBackspace:
		m.handleModifyBackspace()
	case keyNameCtrlN:
		m.handleModifyCtrlN()
	case keyNameCtrlP:
		m.handleModifyCtrlP()
	case keyNameCtrlW:
		m.handleModifyCtrlW()
	default:
		m.handleModifyInput(msg)
	}
	return m, nil
}

func (m *Model) handleModifyTab() {
	if len(m.modifyMatches) == 0 {
		return
	}
	m.modifyIndex = (m.modifyIndex + 1) % len(m.modifyMatches)
	match := m.modifyMatches[m.modifyIndex]
	switch match.Type {
	case matchTypeProject:
		m.modifyInput = "project:" + match.Value + " "
	case matchTypeTag:
		m.modifyInput = "+" + match.Value + " "
	}
	m.updateModifyMatches(m.projects, m.tags)
}

func (m *Model) handleModifyBackspace() {
	if len(m.modifyInput) > 0 {
		m.modifyInput = m.modifyInput[:len(m.modifyInput)-1]
	}
	m.updateModifyMatches(m.projects, m.tags)
}

func (m *Model) handleModifyCtrlN() {
	if len(m.modifyMatches) > 0 {
		m.modifyIndex = (m.modifyIndex + 1) % len(m.modifyMatches)
	}
}

func (m *Model) handleModifyCtrlP() {
	if len(m.modifyMatches) > 0 {
		m.modifyIndex = (m.modifyIndex - 1 + len(m.modifyMatches)) % len(m.modifyMatches)
	}
}

func (m *Model) handleModifyCtrlW() {
	if len(m.modifyInput) > 0 {
		m.modifyInput = deleteLastWord(m.modifyInput)
		m.updateModifyMatches(m.projects, m.tags)
	}
}

func (m *Model) handleModifyInput(msg tea.KeyPressMsg) {
	if len(msg.Text) > 0 {
		m.modifyInput += msg.Text
		m.modifyIndex = 0
		m.updateModifyMatches(m.projects, m.tags)
	}
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

type autocompleteLoadedMsg struct {
	projects []string
	tags     []string
}

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
		taskrc := cfg.TaskRC()

		projects, err := taskwarrior.GetProjects(taskrc)
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := taskwarrior.GetTags(taskrc)
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return configLoadedMsg{cfg: cfg, agents: agents, projects: projects, tags: tags}
	}
}

func loadConfigForAutocomplete() tea.Cmd {
	return func() tea.Msg {
		cfg, _ := config.Load()
		taskrc := ""
		if cfg != nil {
			taskrc = cfg.TaskRC()
		}

		projects, err := taskwarrior.GetProjects(taskrc)
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := taskwarrior.GetTags(taskrc)
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return autocompleteLoadedMsg{projects: projects, tags: tags}
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

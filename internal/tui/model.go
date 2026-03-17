package tui

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/flicktask"
)

// overlayHandled is returned by handleOverlayAction to signal that an action
// was handled (state changed) but no async Cmd needs to run.
var overlayHandled tea.Cmd = func() tea.Msg { return nil }

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
	cursor       int
	selectedUUID string // UUID of task under cursor, survives refresh
	offset       int
	searchInput  textinput.Model

	// Route input
	routeInput   textinput.Model
	routeMatches []agentfs.AgentInfo

	// Text input for overlays
	modifyInput   textinput.Model
	annotateInput textinput.Model

	// Modify input autocomplete
	modifyMatches []modifyMatch
	modifyIndex   int

	// Layout
	width  int
	height int

	// Status
	statusMsg      string
	loading        bool
	teamName       string
	loadingSpinner spinner.Model

	// Heatmap
	heatmapModel heatmapModel
	heatmapReady bool
}

func newTextInput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = placeholder
	return ti
}

func NewModel() Model {
	m := Model{
		state:          stateTaskList,
		filter:         filterPending,
		loading:        true,
		offset:         0,
		searchInput:    newTextInput("project:x +tag priority:H"),
		routeInput:     newTextInput("agent name..."),
		modifyInput:    newTextInput("+tag project:x priority:H"),
		annotateInput:  newTextInput("annotation text"),
		loadingSpinner: spinner.New(spinner.WithSpinner(spinner.MiniDot)),
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfig(), loadTasks(filterPending, ""), m.loadingSpinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case configLoadedMsg, autocompleteLoadedMsg, tasksLoadedMsg, actionResultMsg, execFinishedMsg, heatmapLoadedMsg:
		return m.handleDataMsg(msg)
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.loadingSpinner, cmd = m.loadingSpinner.Update(msg)
		return m, cmd
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.PasteMsg:
		return m.handlePaste(msg)
	}
	return m, nil
}

func (m Model) handleDataMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
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
	case heatmapLoadedMsg:
		if msg.err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.err)
			m.state = stateTaskList
			return m, nil
		}
		m.heatmapModel = msg.model
		m.heatmapReady = true
		return m, nil
	}
	return m, nil
}

func (m *Model) handlePaste(msg tea.PasteMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch m.state {
	case stateSearch:
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.updateSearchMatches(m.projects, m.tags)
	case stateModify:
		m.modifyInput, cmd = m.modifyInput.Update(msg)
		m.updateModifyMatches(m.projects, m.tags)
	case stateAnnotate:
		m.annotateInput, cmd = m.annotateInput.Update(msg)
	case stateRouteInput:
		m.routeInput, cmd = m.routeInput.Update(msg)
		m.updateRouteMatches()
	}
	return m, cmd
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
	case stateRouteInput, stateModify, stateAnnotate, stateConfirmDelete:
		content = m.viewTaskList() // show list behind overlay
	case stateHelp:
		content = m.viewHelp()
	case stateHeatmap:
		content = m.viewHeatmap()
	}

	// Overlays
	switch m.state {
	case stateRouteInput:
		content = m.viewRouteOverlay(content)
	case stateModify:
		content = m.viewModifyOverlay(content)
	case stateAnnotate:
		content = m.viewTextInputOverlay(content, "Annotate Task", "Annotation:", m.annotateInput)
	case stateConfirmDelete:
		content = m.viewConfirmDeleteOverlay(content)
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
	case stateConfirmDelete:
		return m.handleConfirmDeleteKey(msg)
	case stateHeatmap:
		return m.handleHeatmapKey(msg)
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
		if len(m.filtered) > 0 {
			m.selectedUUID = m.filtered[0].UUID
		} else {
			m.selectedUUID = ""
		}
		m.ensureCursorVisible()
	case keyBottom:
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.selectedUUID = m.filtered[m.cursor].UUID
			m.ensureCursorVisible()
		}
	case keyEnter:
		if len(m.filtered) > 0 {
			m.selectedUUID = m.filtered[m.cursor].UUID
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
			t := m.selectedTask()
			if t == nil {
				break
			}
			// Auto-route to manager if one is configured
			if manager := findAgentByRole(m.agents, "manager"); manager != nil {
				return m, routeTask(t.UUID, manager.Name)
			}
			// Fallback: manual agent picker when no manager is configured
			m.state = stateRouteInput
			m.routeInput.SetValue("")
			m.updateRouteMatches()
			return m, m.routeInput.Focus()
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
		m.searchInput.SetValue("")
		if len(m.projects) == 0 || len(m.tags) == 0 {
			return m, tea.Batch(m.searchInput.Focus(), loadConfigForAutocomplete())
		}
		return m, m.searchInput.Focus()
	case keyHelp:
		if m.state == stateHelp {
			m.state = stateTaskList
		} else {
			m.state = stateHelp
		}
	case keyRefresh:
		return m, m.reloadTasks()
	case keyHeatmap:
		m.heatmapReady = false
		m.state = stateHeatmap
		return m, loadHeatmapCmd()
	}
	return m, nil
}

func (m *Model) handleTaskAction(action keyAction) tea.Cmd {
	t := m.selectedTask()
	if t == nil {
		return nil
	}
	if cmd := m.handleOverlayAction(action); cmd != nil {
		return cmd
	}
	switch action {
	case keyExecute:
		return executeTask(t.UUID)
	case keyAddToday:
		return addToToday(t.UUID)
	case keyRemoveToday:
		return removeFromToday(t.UUID)
	case keyToggleNext:
		return toggleNext(t)
	case keyDone:
		return doneTask(t.UUID)
	case keyOpenPR:
		return openPR(t.UUID)
	case keyOpenSession:
		return openSession(t)
	case keyOpenTerm:
		return openTerm(t)
	case keyOpenEditor:
		return openEditor(t)
	case keyCopy:
		return copyTask(t)
	}
	return nil
}

// handleOverlayAction handles actions that open an overlay or change view state.
// Returns a non-nil Cmd (possibly a no-op focus cmd) when an action was handled.
func (m *Model) handleOverlayAction(action keyAction) tea.Cmd {
	switch action {
	case keyModify:
		m.state = stateModify
		m.modifyInput.SetValue("")
		m.updateModifyMatches(m.projects, m.tags)
		return m.modifyInput.Focus()
	case keyAnnotate:
		m.state = stateAnnotate
		m.annotateInput.SetValue("")
		return m.annotateInput.Focus()
	case keyDelete:
		m.state = stateConfirmDelete
		return overlayHandled
	}
	return nil
}

func (m *Model) handleHeatmapKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	action := resolveKey(msg)
	switch action {
	case keyHeatmap, keyEsc:
		m.state = stateTaskList
		return m, nil
	case keyUp, keyDown, keyLeft, keyRight:
		m.heatmapModel.moveCursor(action)
		return m, nil
	}
	return m, nil
}

func (m *Model) handleConfirmDeleteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		t := m.selectedTask()
		m.state = stateTaskList
		if t == nil {
			m.statusMsg = "Error: no task selected to delete"
			return m, nil
		}
		return m, deleteTask(t.UUID)
	default:
		m.state = stateTaskList
	}
	return m, nil
}

func (m *Model) handleSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		m.searchInput.Blur()
		m.state = stateTaskList
		m.cursor = 0
		m.modifyIndex = 0
		return m, m.reloadTasks()
	case keyNameEsc, keyNameCtrlC:
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.state = stateTaskList
		m.cursor = 0
		m.modifyIndex = 0
		return m, m.reloadTasks()
	case keyNameTab:
		m.handleSearchTab()
		return m, nil
	case keyNameCtrlN, keyNameCtrlP:
		m.navigateSearchMatches(s == keyNameCtrlN)
		return m, nil
	default:
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		m.modifyIndex = 0
		m.updateSearchMatches(m.projects, m.tags)
		return m, cmd
	}
}

func (m *Model) handleSearchTab() {
	if len(m.modifyMatches) == 0 {
		return
	}
	m.modifyIndex = (m.modifyIndex + 1) % len(m.modifyMatches)
	match := m.modifyMatches[m.modifyIndex]
	switch match.Type {
	case matchTypeProject:
		m.searchInput.SetValue("project:" + match.Value + " ")
		m.searchInput.CursorEnd()
	case matchTypeTag:
		m.searchInput.SetValue("+" + match.Value + " ")
		m.searchInput.CursorEnd()
	default:
		m.searchInput.SetValue(m.searchInput.Value() + match.Value + " ")
		m.searchInput.CursorEnd()
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

func (m *Model) handleModifyKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		t := m.selectedTask()
		m.modifyInput.Blur()
		if t != nil && m.modifyInput.Value() != "" {
			m.state = stateTaskList
			m.modifyIndex = 0
			return m, modifyTask(t.UUID, m.modifyInput.Value())
		}
		m.state = stateTaskList
		m.modifyIndex = 0
	case keyNameEsc:
		m.modifyInput.Blur()
		m.state = stateTaskList
		m.modifyIndex = 0
	case keyNameTab:
		m.handleModifyTab()
	case keyNameCtrlN:
		m.handleModifyCtrlN()
	case keyNameCtrlP:
		m.handleModifyCtrlP()
	default:
		var cmd tea.Cmd
		m.modifyInput, cmd = m.modifyInput.Update(msg)
		m.modifyIndex = 0
		m.updateModifyMatches(m.projects, m.tags)
		return m, cmd
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
		m.modifyInput.SetValue("project:" + match.Value + " ")
		m.modifyInput.CursorEnd()
	case matchTypeTag:
		m.modifyInput.SetValue("+" + match.Value + " ")
		m.modifyInput.CursorEnd()
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

func (m *Model) handleAnnotateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		m.annotateInput.Blur()
		t := m.selectedTask()
		if t != nil && m.annotateInput.Value() != "" {
			m.state = stateTaskList
			return m, annotateTask(t.UUID, m.annotateInput.Value())
		}
		m.state = stateTaskList
	case keyNameEsc:
		m.annotateInput.Blur()
		m.state = stateTaskList
	default:
		var cmd tea.Cmd
		m.annotateInput, cmd = m.annotateInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *Model) handleRouteKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := msg.String()
	switch s {
	case keyNameEnter:
		m.routeInput.Blur()
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
		m.routeInput.Blur()
		m.state = stateTaskList
	case keyNameTab:
		if len(m.routeMatches) > 0 {
			m.routeInput.SetValue(m.routeMatches[0].Name)
			m.routeInput.CursorEnd()
			m.updateRouteMatches()
		}
	default:
		var cmd tea.Cmd
		m.routeInput, cmd = m.routeInput.Update(msg)
		m.updateRouteMatches()
		return m, cmd
	}
	return m, nil
}

func (m *Model) updateRouteMatches() {
	q := strings.ToLower(m.routeInput.Value())
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
	m.syncSelectedUUID()
	m.ensureCursorVisible()
}

// syncSelectedUUID keeps selectedUUID in sync with the current cursor position.
func (m *Model) syncSelectedUUID() {
	if m.cursor >= 0 && m.cursor < len(m.filtered) {
		m.selectedUUID = m.filtered[m.cursor].UUID
	}
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
	prevUUID := m.selectedUUID

	m.filtered = nil
	for _, t := range m.tasks {
		switch m.filter {
		case filterPending:
			if t.IsActive() {
				continue
			}
		case filterToday:
			if !t.IsToday() || t.IsActive() {
				continue
			}
		case filterActive:
			if t.Start == "" {
				continue
			}
		}
		m.filtered = append(m.filtered, t)
	}
	// Sort by entry descending (newest tasks first).
	sort.Slice(m.filtered, func(i, j int) bool {
		return m.filtered[i].Entry > m.filtered[j].Entry
	})

	if m.restoreCursorByUUID(prevUUID) {
		return
	}

	// Fallback: clamp cursor (task was filtered out or deleted)
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	m.syncSelectedUUID()
	m.offset = 0
}

// restoreCursorByUUID repositions the cursor to the task with the given UUID.
// Returns true if found; caller should return early to skip fallback clamping.
func (m *Model) restoreCursorByUUID(uuid string) bool {
	if uuid == "" {
		return false
	}
	for i, t := range m.filtered {
		if t.UUID == uuid {
			m.cursor = i
			m.offset = 0
			m.ensureCursorVisible()
			return true
		}
	}
	return false
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
		agents, err := agentfs.Discover(cfg.TeamPath())
		if err != nil {
			log.Printf("failed to discover agents: %v", err)
		}

		projects, err := flicktask.GetProjects()
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := flicktask.GetTags()
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return configLoadedMsg{cfg: cfg, agents: agents, projects: projects, tags: tags}
	}
}

func loadConfigForAutocomplete() tea.Cmd {
	return func() tea.Msg {
		projects, err := flicktask.GetProjects()
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := flicktask.GetTags()
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return autocompleteLoadedMsg{projects: projects, tags: tags}
	}
}

func (m *Model) reloadTasks() tea.Cmd {
	m.loading = true
	return loadTasks(m.filter, m.searchInput.Value())
}

func loadTasks(filter filterMode, search string) tea.Cmd {
	return func() tea.Msg {
		completed := filter == filterCompleted

		var ftTasks []flicktask.Task
		var err error
		if search != "" {
			ftTasks, err = flicktask.FindTasks(strings.Fields(search), completed)
		} else {
			ftTasks, err = flicktask.ExportAll(completed)
		}
		if err != nil {
			return tasksLoadedMsg{err: fmt.Errorf("flicktask: %w", err)}
		}

		tasks := make([]Task, len(ftTasks))
		for i, t := range ftTasks {
			tasks[i] = Task{t}
		}
		return tasksLoadedMsg{tasks: tasks}
	}
}

// findAgentByRole returns the first agent with the given role, or nil.
func findAgentByRole(agents []agentfs.AgentInfo, role string) *agentfs.AgentInfo {
	for i := range agents {
		if agents[i].Role == role {
			return &agents[i]
		}
	}
	return nil
}

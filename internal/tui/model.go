package tui

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"github.com/tta-lab/ttal-cli/internal/agentfs"
	"github.com/tta-lab/ttal-cli/internal/config"
	"github.com/tta-lab/ttal-cli/internal/pipeline"
	"github.com/tta-lab/ttal-cli/internal/taskwarrior"
)

// refreshInterval is how often the TUI polls taskwarrior for updates.
const refreshInterval = 500 * time.Millisecond

// refreshTickMsg signals that it's time to poll taskwarrior for updates.
type refreshTickMsg struct{}

// scheduleRefresh returns a Cmd that sends a refreshTickMsg after refreshInterval.
func scheduleRefresh() tea.Cmd {
	return tea.Tick(refreshInterval, func(time.Time) tea.Msg {
		return refreshTickMsg{}
	})
}

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
	cfg      *config.Config
	projects []string
	tags     []string

	// Task list cursor
	cursor       int
	selectedUUID string // UUID of task under cursor, survives refresh
	offset       int
	searchInput  textinput.Model

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

	// Child cache for detail view
	childrenCache map[string][]Task // parent UUID → loaded children

	// Active filter columns
	agentEmojiByName map[string]string // agent name → emoji (from agentfs)
	pipelineCfg      *pipeline.Config  // pipeline definitions (for stage resolution)

	// Key bindings and help
	keys         KeyMap
	helpModel    help.Model
	helpViewport viewport.Model
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
		modifyInput:    newTextInput("+tag project:x priority:H"),
		annotateInput:  newTextInput("annotation text"),
		loadingSpinner: spinner.New(spinner.WithSpinner(spinner.MiniDot)),
		childrenCache:  make(map[string][]Task),
		keys:           DefaultKeyMap(),
		helpModel:      help.New(),
		helpViewport:   viewport.New(),
	}
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(loadConfig(), loadTasks(filterPending, ""), m.loadingSpinner.Tick, scheduleRefresh())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.state == stateHelp {
			m.helpViewport.SetWidth(m.width)
			m.helpViewport.SetHeight(m.height - 3)
		}
		return m, nil
	case configLoadedMsg, autocompleteLoadedMsg, tasksLoadedMsg,
		actionResultMsg, execFinishedMsg, heatmapLoadedMsg, childrenLoadedMsg:
		return m.handleDataMsg(msg)
	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.loadingSpinner, cmd = m.loadingSpinner.Update(msg)
		return m, cmd
	case refreshTickMsg:
		// Only auto-refresh when the task list is visible (not in overlays, detail, help, or heatmap)
		if m.state != stateTaskList {
			return m, scheduleRefresh()
		}
		return m, tea.Batch(loadTasks(m.filter, m.searchInput.Value()), scheduleRefresh())
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
		m.projects = msg.projects
		m.tags = msg.tags
		m.agentEmojiByName = msg.agentEmojiByName
		m.pipelineCfg = msg.pipelineCfg
		if msg.cfg != nil {
			m.teamName = msg.cfg.TeamName()
		}
		return m, nil
	case autocompleteLoadedMsg:
		m.projects = msg.projects
		m.tags = msg.tags
		return m, nil
	case tasksLoadedMsg:
		return m.handleTasksLoaded(msg)
	case childrenLoadedMsg:
		return m.handleChildrenLoaded(msg)
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

func (m *Model) handleTasksLoaded(msg tasksLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.statusMsg = "Task load error: " + msg.err.Error()
	}
	m.tasks = msg.tasks
	m.applyFilter()
	return m, nil
}

func (m *Model) handleChildrenLoaded(msg childrenLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.statusMsg = fmt.Sprintf("Load children [%s]: %s", msg.parentUUID[:8], msg.err.Error())
		return m, nil
	}
	m.childrenCache[msg.parentUUID] = msg.children
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
	case stateModify, stateAnnotate, stateConfirmDelete:
		content = m.viewTaskList() // show list behind overlay
	case stateHelp:
		content = m.viewHelp()
	case stateHeatmap:
		content = m.viewHeatmap()
	}

	// Overlays
	switch m.state {
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
	case stateHelp:
		return m.handleHelpKey(msg)
	}

	switch {
	case key.Matches(msg, m.keys.Quit):
		if m.state != stateTaskList {
			m.state = stateTaskList
			return m, nil
		}
		return m, tea.Quit
	case key.Matches(msg, m.keys.Esc):
		if m.state != stateTaskList {
			m.state = stateTaskList
		}
		return m, nil
	}

	if model, cmd, handled := m.handleTaskListKey(msg); handled {
		return model, cmd
	}
	if handled := m.handleNavigation(msg); handled {
		return m, nil
	}
	return m.handleAction(msg)
}

// handleTaskListKey handles Enter (task list only).
// Returns (model, cmd, true) when handled, (nil, nil, false) to fall through.
func (m *Model) handleTaskListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Enter):
		if len(m.filtered) == 0 {
			return m, nil, true
		}
		t := &m.filtered[m.cursor]
		m.selectedUUID = t.UUID
		m.state = stateTaskDetail
		if _, ok := m.childrenCache[m.selectedUUID]; !ok {
			return m, loadChildren(m.selectedUUID), true
		}
		return m, nil, true
	}
	return nil, nil, false
}

// handleNavigation handles cursor movement keys. Returns true when handled.
func (m *Model) handleNavigation(msg tea.KeyPressMsg) bool {
	switch {
	case key.Matches(msg, m.keys.Up):
		m.moveCursor(-1)
	case key.Matches(msg, m.keys.Down):
		m.moveCursor(1)
	case key.Matches(msg, m.keys.PageDown):
		m.moveCursor(m.visibleRows())
	case key.Matches(msg, m.keys.PageUp):
		m.moveCursor(-m.visibleRows())
	case key.Matches(msg, m.keys.HalfPageDown):
		m.moveCursor(m.visibleRows() / 2)
	case key.Matches(msg, m.keys.HalfPageUp):
		m.moveCursor(-m.visibleRows() / 2)
	case key.Matches(msg, m.keys.Top):
		m.cursor = 0
		if len(m.filtered) > 0 {
			m.selectedUUID = m.filtered[0].UUID
		} else {
			m.selectedUUID = ""
		}
		m.ensureCursorVisible()
	case key.Matches(msg, m.keys.Bottom):
		if len(m.filtered) > 0 {
			m.cursor = len(m.filtered) - 1
			m.selectedUUID = m.filtered[m.cursor].UUID
			m.ensureCursorVisible()
		}
	default:
		return false
	}
	return true
}

// handleAction handles global and task-scoped actions.
func (m *Model) handleAction(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.FilterNext):
		m.filter = m.filter.Next()
		m.cursor = 0
		return m, m.reloadTasks()
	case key.Matches(msg, m.keys.FilterPrev):
		m.filter = m.filter.Prev()
		m.cursor = 0
		return m, m.reloadTasks()
	case key.Matches(msg, m.keys.Search):
		m.state = stateSearch
		m.searchInput.SetValue("")
		if len(m.projects) == 0 || len(m.tags) == 0 {
			return m, tea.Batch(m.searchInput.Focus(), loadConfigForAutocomplete())
		}
		return m, m.searchInput.Focus()
	case key.Matches(msg, m.keys.Help):
		m.state = stateHelp
		m.helpModel.ShowAll = true
		m.helpModel.SetWidth(m.width - 4)
		m.helpViewport.SetWidth(m.width)
		m.helpViewport.SetHeight(m.height - 3)
		m.helpViewport.SetContent(m.helpModel.FullHelpView(m.keys.FullHelp()))
		m.helpViewport.GotoTop()
		return m, nil
	case key.Matches(msg, m.keys.Refresh):
		return m, m.reloadTasks()
	case key.Matches(msg, m.keys.Heatmap):
		m.heatmapReady = false
		m.state = stateHeatmap
		return m, loadHeatmapCmd()
	}
	return m.handleTaskAction(msg)
}

// handleTaskAction handles actions that operate on the selected task.
func (m *Model) handleTaskAction(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	t := m.selectedTask()
	if t == nil {
		return m, nil
	}
	switch {
	case key.Matches(msg, m.keys.Advance):
		return m, advanceTask(t.UUID)
	case key.Matches(msg, m.keys.Done):
		return m, doneTask(t.UUID)
	case key.Matches(msg, m.keys.Modify):
		m.state = stateModify
		m.modifyInput.SetValue("")
		m.updateModifyMatches(m.projects, m.tags)
		return m, m.modifyInput.Focus()
	case key.Matches(msg, m.keys.Annotate):
		m.state = stateAnnotate
		m.annotateInput.SetValue("")
		return m, m.annotateInput.Focus()
	case key.Matches(msg, m.keys.Delete):
		m.state = stateConfirmDelete
		return m, overlayHandled
	case key.Matches(msg, m.keys.AddToday):
		return m, addToToday(t.UUID)
	case key.Matches(msg, m.keys.RemoveToday):
		return m, removeFromToday(t.UUID)
	case key.Matches(msg, m.keys.ToggleNext):
		return m, toggleNext(t)
	case key.Matches(msg, m.keys.OpenPR):
		return m, openPR(t.UUID)
	case key.Matches(msg, m.keys.OpenSession):
		return m, openSession(t, m.cfg)
	case key.Matches(msg, m.keys.OpenTerm):
		return m, openTerm(t)
	case key.Matches(msg, m.keys.OpenEditor):
		return m, openEditor(t)
	case key.Matches(msg, m.keys.Copy):
		return m, copyTask(t)
	}
	return m, nil
}

func (m *Model) handleHelpKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit), key.Matches(msg, m.keys.Esc), key.Matches(msg, m.keys.Help):
		m.state = stateTaskList
		return m, nil
	default:
		var cmd tea.Cmd
		m.helpViewport, cmd = m.helpViewport.Update(msg)
		return m, cmd
	}
}

func (m *Model) handleHeatmapKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Heatmap), key.Matches(msg, m.keys.Esc):
		m.state = stateTaskList
		return m, nil
	case key.Matches(msg, m.keys.Up):
		m.heatmapModel.handleKey("up")
	case key.Matches(msg, m.keys.Down):
		m.heatmapModel.handleKey("down")
	case msg.String() == "h", msg.String() == "left":
		m.heatmapModel.handleKey("left")
	case msg.String() == "l", msg.String() == "right":
		m.heatmapModel.handleKey("right")
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

// rootSortLess returns a sort comparator for root tasks.
// Completed tasks sort by End descending; all others sort by urgency descending.
func (m *Model) rootSortLess(roots []Task) func(i, j int) bool {
	if m.filter == filterCompleted {
		return func(i, j int) bool { return roots[i].End > roots[j].End }
	}
	return func(i, j int) bool { return roots[i].Urgency > roots[j].Urgency }
}

func (m *Model) applyFilter() {
	prevUUID := m.selectedUUID

	// Phase 1: filter root tasks
	roots := make([]Task, 0, len(m.tasks))
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
		roots = append(roots, t)
	}
	sort.Slice(roots, m.rootSortLess(roots))
	m.filtered = roots

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
	cfg              *config.Config
	projects         []string
	tags             []string
	agentEmojiByName map[string]string
	pipelineCfg      *pipeline.Config
	err              error
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

type childrenLoadedMsg struct {
	parentUUID string
	children   []Task
	err        error
}

// Commands

func loadConfig() tea.Cmd {
	return func() tea.Msg {
		cfg, err := config.Load()
		if err != nil {
			return configLoadedMsg{err: fmt.Errorf("load config: %w", err)}
		}

		projects, err := taskwarrior.GetProjects()
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := taskwarrior.GetTags()
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		// Agent emojis from frontmatter (depends on cfg.TeamPath())
		agentEmojiMap := make(map[string]string)
		if cfg != nil {
			if agents, err := agentfs.Discover(cfg.TeamPath()); err != nil {
				log.Printf("failed to discover agents for active view: %v", err)
			} else {
				for _, a := range agents {
					agentEmojiMap[a.Name] = a.Emoji
				}
			}
		}

		// Pipeline config
		var pipeCfg *pipeline.Config
		if pc, err := pipeline.Load(config.DefaultConfigDir()); err != nil {
			log.Printf("failed to load pipeline config for active view: %v", err)
		} else {
			pipeCfg = pc
		}

		return configLoadedMsg{
			cfg: cfg, projects: projects, tags: tags,
			agentEmojiByName: agentEmojiMap, pipelineCfg: pipeCfg,
		}
	}
}

func loadConfigForAutocomplete() tea.Cmd {
	return func() tea.Msg {
		projects, err := taskwarrior.GetProjects()
		if err != nil {
			log.Printf("failed to load projects for autocomplete: %v", err)
		}
		tags, err := taskwarrior.GetTags()
		if err != nil {
			log.Printf("failed to load tags for autocomplete: %v", err)
		}

		return autocompleteLoadedMsg{projects: projects, tags: tags}
	}
}

func loadChildren(parentUUID string) tea.Cmd {
	return func() tea.Msg {
		children, err := taskwarrior.GetChildren(parentUUID)
		return childrenLoadedMsg{parentUUID: parentUUID, children: children, err: err}
	}
}

func (m *Model) reloadTasks() tea.Cmd {
	m.loading = true
	return loadTasks(m.filter, m.searchInput.Value())
}

// buildLoadTasksArgs returns the taskwarrior args for the given filter and search.
// Extracted for testability.
func buildLoadTasksArgs(filter filterMode, search string) []string {
	var args []string
	switch filter {
	case filterPending, filterToday:
		args = append(args, "status:pending")
		if taskwarrior.IsFork() {
			args = append(args, "parent_id:") // root tasks only (fork feature)
		}
	case filterActive:
		args = append(args, "status:pending")
		if taskwarrior.IsFork() {
			args = append(args, "parent_id:") // root tasks only (fork feature)
		}
	case filterCompleted:
		args = append(args, "status:completed")
	}

	// Pass search as raw taskwarrior filter args
	if search != "" {
		args = append(args, strings.Fields(search)...)
	}

	args = append(args, "export")
	return args
}

func loadTasks(filter filterMode, search string) tea.Cmd {
	return func() tea.Msg {
		args := buildLoadTasksArgs(filter, search)
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

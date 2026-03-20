package tui

type viewState int

const (
	stateTaskList viewState = iota
	stateTaskDetail
	stateSearch
	stateModify
	stateAnnotate
	stateHelp
	stateConfirmDelete
	stateHeatmap
)

package tui

type viewState int

const (
	stateTaskList viewState = iota
	stateTaskDetail
	stateRouteInput
	stateSearch
	stateModify
	stateAnnotate
	stateHelp
	stateConfirmDelete
)

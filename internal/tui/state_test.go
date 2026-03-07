package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewState(t *testing.T) {
	tests := []struct {
		input    viewState
		expected int
	}{
		{stateTaskList, 0},
		{stateTaskDetail, 1},
		{stateRouteInput, 2},
		{stateSearch, 3},
		{stateModify, 4},
		{stateAnnotate, 5},
		{stateHelp, 6},
	}
	for _, tt := range tests {
		t.Run("state", func(t *testing.T) {
			result := int(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

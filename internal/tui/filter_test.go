package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterModeNext(t *testing.T) {
	tests := []struct {
		input    filterMode
		expected filterMode
	}{
		{filterPending, filterToday},
		{filterToday, filterActive},
		{filterActive, filterCompleted},
		{filterCompleted, filterPending},
	}
	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := tt.input.Next()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterModePrev(t *testing.T) {
	tests := []struct {
		input    filterMode
		expected filterMode
	}{
		{filterPending, filterCompleted},
		{filterToday, filterPending},
		{filterActive, filterToday},
		{filterCompleted, filterActive},
	}
	for _, tt := range tests {
		t.Run(tt.input.String(), func(t *testing.T) {
			result := tt.input.Prev()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterModeString(t *testing.T) {
	tests := []struct {
		input    filterMode
		expected string
	}{
		{filterPending, "pending"},
		{filterToday, "today"},
		{filterActive, "active"},
		{filterCompleted, "completed"},
		{filterMode(99), "pending"},
	}
	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := tt.input.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

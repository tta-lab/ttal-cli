package tui

import (
	"strings"
	"testing"
	"time"
)

func TestRenderHeatmap_Empty(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	result := RenderHeatmap(map[time.Time]int{}, now)

	if !strings.Contains(result, "Jan") {
		t.Error("missing month label Jan")
	}
	if !strings.Contains(result, "Mar") {
		t.Error("missing month label Mar")
	}
	if !strings.Contains(result, "Mon") {
		t.Error("missing day label Mon")
	}
	if !strings.Contains(result, "0 tasks completed in the last year") {
		t.Errorf("missing summary line, got:\n%s", result)
	}
}

func TestRenderHeatmap_WithData(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	counts := map[time.Time]int{
		now:                     5,
		now.AddDate(0, 0, -7):   2,
		now.AddDate(0, 0, -100): 1,
	}
	result := RenderHeatmap(counts, now)

	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	// month header + 7 data rows + summary = 9 lines minimum
	if len(lines) < 9 {
		t.Errorf("expected ≥9 lines, got %d:\n%s", len(lines), result)
	}
	if !strings.Contains(result, "8 tasks completed in the last year") {
		t.Errorf("unexpected summary, got:\n%s", result)
	}
}

func TestBuildGrid_TodayCursorPosition(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC) // Thursday
	grid := buildGrid(map[time.Time]int{}, now)

	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	found := false
	for week := 0; week < 53; week++ {
		for day := 0; day < 7; day++ {
			if grid.dates[week][day].Equal(todayDate) {
				found = true
				// Thursday = weekday 4
				if day != 4 {
					t.Errorf("expected today at day row 4 (Thu), got %d", day)
				}
			}
		}
	}
	if !found {
		t.Error("today not found in grid")
	}
}

func TestColorBucket(t *testing.T) {
	cases := []struct {
		count, max int
		want       int
	}{
		{0, 10, 0},
		{0, 0, 0},
		{1, 10, 1},
		{5, 10, 2},
		{8, 10, 3},
		{10, 10, 4},
	}
	for _, c := range cases {
		got := colorBucket(c.count, c.max)
		if got != c.want {
			t.Errorf("colorBucket(%d, %d) = %d, want %d", c.count, c.max, got, c.want)
		}
	}
}

func TestHeatmapModel_MoveCursor(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	grid := buildGrid(map[time.Time]int{}, now)
	m := heatmapModel{grid: grid, cursorX: 26, cursorY: 3}

	m.moveCursor(keyLeft)
	if m.cursorX != 25 {
		t.Errorf("expected cursorX=25 after keyLeft, got %d", m.cursorX)
	}

	m.moveCursor(keyRight)
	if m.cursorX != 26 {
		t.Errorf("expected cursorX=26 after keyRight, got %d", m.cursorX)
	}

	m.moveCursor(keyUp)
	if m.cursorY != 2 {
		t.Errorf("expected cursorY=2 after keyUp, got %d", m.cursorY)
	}

	m.moveCursor(keyDown)
	if m.cursorY != 3 {
		t.Errorf("expected cursorY=3 after keyDown, got %d", m.cursorY)
	}
}

func TestHeatmapModel_MoveCursor_WrapVertical(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	grid := buildGrid(map[time.Time]int{}, now)
	m := heatmapModel{grid: grid, cursorX: 0, cursorY: 0}

	// Moving up from row 0 should wrap to row 6, but guard future dates may revert.
	// Use week 0 which has old dates, so no future guard.
	m.moveCursor(keyUp)
	if m.cursorY != 6 {
		t.Errorf("expected cursorY=6 after wrapping up from 0, got %d", m.cursorY)
	}
}

func TestHeatmapModel_MoveCursor_ClampHorizontal(t *testing.T) {
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	grid := buildGrid(map[time.Time]int{}, now)
	m := heatmapModel{grid: grid, cursorX: 0, cursorY: 0}

	// Can't go left past week 0.
	m.moveCursor(keyLeft)
	if m.cursorX != 0 {
		t.Errorf("expected cursorX to stay 0 at left boundary, got %d", m.cursorX)
	}

	// Can't go right past week 52.
	m.cursorX = 52
	m.cursorY = 0 // week 52 day 0 = Sun 2026-03-08, not future
	m.moveCursor(keyRight)
	if m.cursorX != 52 {
		t.Errorf("expected cursorX to stay 52 at right boundary, got %d", m.cursorX)
	}
}

func TestHeatmapModel_MoveCursor_FutureDateGuard(t *testing.T) {
	// now = 2026-03-12 (Thu). Today is at grid.todayWeek=52, grid.todayDay=4.
	// Moving down from today lands on (52, 5) = Fri 2026-03-13 = future — should revert.
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	grid := buildGrid(map[time.Time]int{}, now)
	m := heatmapModel{grid: grid, cursorX: grid.todayWeek, cursorY: grid.todayDay}

	m.moveCursor(keyDown)
	if m.cursorX != grid.todayWeek || m.cursorY != grid.todayDay {
		t.Errorf("expected cursor to revert to today (%d,%d), got (%d,%d)",
			grid.todayWeek, grid.todayDay, m.cursorX, m.cursorY)
	}
}

func TestBuildGrid_TodayPosition(t *testing.T) {
	// now = 2026-03-12 (Thu): todayWeek and todayDay should be set in grid.
	now := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	grid := buildGrid(map[time.Time]int{}, now)

	todayDate := time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC)
	if !grid.dates[grid.todayWeek][grid.todayDay].Equal(todayDate) {
		t.Errorf("grid.dates[%d][%d] = %v, want %v",
			grid.todayWeek, grid.todayDay, grid.dates[grid.todayWeek][grid.todayDay], todayDate)
	}
	// Thursday = weekday 4
	if grid.todayDay != 4 {
		t.Errorf("expected todayDay=4 (Thu), got %d", grid.todayDay)
	}
}

package tui

import (
	"fmt"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/tta-lab/ttal-cli/internal/today"
)

// GitHub green color scale — 5 levels from empty to high.
var heatmapColors = []color.Color{
	lipgloss.Color("#161b22"), // 0: empty
	lipgloss.Color("#0e4429"), // 1: low
	lipgloss.Color("#006d32"), // 2: medium-low
	lipgloss.Color("#26a641"), // 3: medium-high
	lipgloss.Color("#39d353"), // 4: high
}

// heatmapCellStyles[i] is the precomputed lipgloss style for color bucket i.
// Allocated once at package init to avoid per-cell allocations in renderGrid.
var heatmapCellStyles [5]lipgloss.Style

func init() {
	for i, c := range heatmapColors {
		heatmapCellStyles[i] = lipgloss.NewStyle().Foreground(c)
	}
}

// heatmapGrid holds the precomputed 53×7 grid data.
type heatmapGrid struct {
	cells     [53][7]int
	dates     [53][7]time.Time
	max       int
	total     int
	today     time.Time
	todayWeek int // week column of today (0-52)
	todayDay  int // day row of today (0-6)
}

// buildGrid builds the 53×7 grid from completed counts.
// Columns are weeks (0=oldest), rows are days of week (0=Sun, 6=Sat).
// Use UTC consistently — taskwarrior dates parse as UTC.
func buildGrid(counts map[time.Time]int, now time.Time) heatmapGrid {
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	start := todayDate.AddDate(0, 0, -364)
	for start.Weekday() != time.Sunday {
		start = start.AddDate(0, 0, -1)
	}

	var g heatmapGrid
	g.today = todayDate

	for week := 0; week < 53; week++ {
		for day := 0; day < 7; day++ {
			date := start.AddDate(0, 0, week*7+day)
			g.dates[week][day] = date
			if date.Equal(todayDate) {
				g.todayWeek = week
				g.todayDay = day
			}
			if !date.After(todayDate) {
				count := counts[date]
				g.cells[week][day] = count
				g.total += count
				if count > g.max {
					g.max = count
				}
			}
		}
	}

	return g
}

// colorBucket maps a count to a 0-4 color index.
// Formula 1+(count*3)/max yields range 1-4; count==max yields exactly 4.
func colorBucket(count, max int) int {
	if count == 0 || max == 0 {
		return 0
	}
	return 1 + (count*3)/max
}

// dayLabel returns the left label for a given row (GitHub style: Mon, Wed, Fri only).
func dayLabel(row int) string {
	switch row {
	case 1:
		return "Mon "
	case 3:
		return "Wed "
	case 5:
		return "Fri "
	default:
		return "    "
	}
}

// RenderHeatmap returns a compact GitHub-style heatmap string (for CLI use, no cursor).
// Exported so cmd/today.go can call tui.RenderHeatmap().
func RenderHeatmap(counts map[time.Time]int, now time.Time) string {
	grid := buildGrid(counts, now)
	var sb strings.Builder
	sb.WriteString(renderGrid(grid, -1, -1))
	fmt.Fprintf(&sb, "%d tasks completed in the last year\n", grid.total)
	return sb.String()
}

// renderGrid renders the 53×7 grid. cursorX/cursorY = -1 means no cursor highlight.
func renderGrid(grid heatmapGrid, cursorX, cursorY int) string {
	var sb strings.Builder

	sb.WriteString(renderMonthLabels(grid))
	sb.WriteByte('\n')

	for row := 0; row < 7; row++ {
		sb.WriteString(dayLabel(row))
		for week := 0; week < 53; week++ {
			date := grid.dates[week][row]
			isFuture := date.After(grid.today)
			count := grid.cells[week][row]

			bucket := 0
			if !isFuture {
				bucket = colorBucket(count, grid.max)
			}

			style := heatmapCellStyles[bucket]
			if week == cursorX && row == cursorY {
				style = style.Reverse(true)
			}
			sb.WriteString(style.Render("██"))
		}
		sb.WriteByte('\n')
	}

	return sb.String()
}

// renderMonthLabels returns the month header row (plain text, no ANSI).
func renderMonthLabels(grid heatmapGrid) string {
	const dayLabelWidth = 4
	const weeksTotal = 53
	buf := []byte(strings.Repeat(" ", dayLabelWidth+weeksTotal*2))

	var prevMonth time.Month
	for week := 0; week < weeksTotal; week++ {
		d := grid.dates[week][0]
		if d.IsZero() {
			continue
		}
		m := d.Month()
		if week == 0 || m != prevMonth {
			prevMonth = m
			label := m.String()[:3]
			pos := dayLabelWidth + week*2
			for i := 0; i < len(label) && pos+i < len(buf); i++ {
				buf[pos+i] = label[i]
			}
		}
	}

	return string(buf)
}

// heatmapModel wraps the grid with cursor state for TUI use.
type heatmapModel struct {
	grid    heatmapGrid
	cursorX int // week column (0-52)
	cursorY int // day row (0-6)
}

// view renders the grid with cursor highlight, status line, and summary.
func (h heatmapModel) view() string {
	var sb strings.Builder
	sb.WriteString(renderGrid(h.grid, h.cursorX, h.cursorY))
	sb.WriteByte('\n')

	// Status line for selected cell.
	date := h.grid.dates[h.cursorX][h.cursorY]
	if date.IsZero() || date.After(h.grid.today) {
		sb.WriteString("—")
	} else {
		count := h.grid.cells[h.cursorX][h.cursorY]
		var countStr string
		switch count {
		case 0:
			countStr = "no tasks"
		case 1:
			countStr = "1 task completed"
		default:
			countStr = fmt.Sprintf("%d tasks completed", count)
		}
		sb.WriteString(styleDim.Render(date.Format("Mon Jan 2, 2006") + ": " + countStr))
	}

	sb.WriteByte('\n')
	sb.WriteString(styleDim.Render(fmt.Sprintf("%d tasks completed in the last year", h.grid.total)))

	return sb.String()
}

// moveCursor handles arrow key navigation with future-date guard.
func (h *heatmapModel) moveCursor(action keyAction) {
	prevX, prevY := h.cursorX, h.cursorY

	switch action {
	case keyUp:
		h.cursorY--
		if h.cursorY < 0 {
			h.cursorY = 6
		}
	case keyDown:
		h.cursorY++
		if h.cursorY > 6 {
			h.cursorY = 0
		}
	case keyLeft:
		if h.cursorX > 0 {
			h.cursorX--
		}
	case keyRight:
		if h.cursorX < 52 {
			h.cursorX++
		}
	}

	// Guard: revert if landed on a future date.
	if h.grid.dates[h.cursorX][h.cursorY].After(h.grid.today) {
		h.cursorX, h.cursorY = prevX, prevY
	}
}

// heatmapLoadedMsg is returned by the async heatmap loading command.
type heatmapLoadedMsg struct {
	model heatmapModel
	err   error
}

// loadHeatmapCmd returns a tea.Cmd that loads heatmap data asynchronously.
func loadHeatmapCmd() tea.Cmd {
	return func() tea.Msg {
		counts, err := today.CompletedCounts()
		if err != nil {
			return heatmapLoadedMsg{err: err}
		}
		grid := buildGrid(counts, time.Now())
		// Initialize cursor at today's position (stored in grid during buildGrid).
		model := heatmapModel{
			grid:    grid,
			cursorX: grid.todayWeek,
			cursorY: grid.todayDay,
		}
		return heatmapLoadedMsg{model: model}
	}
}

// viewHeatmap renders the heatmap view with title and footer.
func (m Model) viewHeatmap() string {
	if !m.heatmapReady {
		return "Loading heatmap..."
	}

	title := styleHeatmapTitle.Render("Task Completion Heatmap — Past Year")
	footer := styleDim.Render("←↑↓→ navigate  h/esc back")

	return fmt.Sprintf("%s\n\n%s\n\n%s", title, m.heatmapModel.view(), footer)
}

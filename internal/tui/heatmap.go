package tui

import (
	"fmt"
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/heatmap"
	"github.com/tta-lab/ttal-cli/internal/today"
)

// GitHub green color scale — uses image/color.RGBA (ntcharts WithColorScale expects []color.Color)
var greenScale = []color.Color{
	color.RGBA{R: 22, G: 27, B: 34, A: 255},  // #161b22 — empty
	color.RGBA{R: 14, G: 68, B: 41, A: 255},  // #0e4429 — low
	color.RGBA{R: 0, G: 109, B: 50, A: 255},  // #006d32 — medium-low
	color.RGBA{R: 38, G: 166, B: 65, A: 255}, // #26a641 — medium-high
	color.RGBA{R: 57, G: 211, B: 83, A: 255}, // #39d353 — high
}

// heatmapLoadedMsg is returned by the async heatmap loading command.
type heatmapLoadedMsg struct {
	model heatmap.Model
	total int
	err   error
}

// loadHeatmapCmd returns a tea.Cmd that loads heatmap data asynchronously.
func loadHeatmapCmd(width, height int) tea.Cmd {
	return func() tea.Msg {
		counts, err := today.CompletedCounts()
		if err != nil {
			return heatmapLoadedMsg{err: err}
		}

		// Grid: 53 columns (weeks) × 7 rows (days of week)
		// 53 because a 365-day window can span 53 Sundays
		hm := heatmap.New(53, 7,
			heatmap.WithColorScale(greenScale),
			heatmap.WithAutoValueRange(),
		)
		hm.Resize(width, height)

		now := time.Now()
		// Use UTC consistently — taskwarrior dates parse as UTC
		todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Find the Sunday that starts the first column
		start := todayDate.AddDate(0, 0, -364)
		for start.Weekday() != time.Sunday {
			start = start.AddDate(0, 0, -1)
		}

		total := 0
		for week := 0; week < 53; week++ {
			for day := 0; day < 7; day++ {
				date := start.AddDate(0, 0, week*7+day)
				if date.After(todayDate) {
					continue
				}
				count := counts[date]
				if count > 0 {
					hm.Push(heatmap.NewHeatPointInt(week, day, float64(count)))
					total += count
				}
			}
		}

		hm.Draw()
		return heatmapLoadedMsg{model: hm, total: total}
	}
}

// viewHeatmap renders the heatmap view with title and summary.
func (m Model) viewHeatmap() string {
	if !m.heatmapReady {
		return "Loading heatmap..."
	}

	title := lipgloss.NewStyle().Bold(true).Render("Task Completion Heatmap — Past Year")

	summary := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(fmt.Sprintf("%d tasks completed in the last year", m.heatmapTotal))

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("h/esc  back")

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", title, m.heatmapModel.View(), summary, footer)
}

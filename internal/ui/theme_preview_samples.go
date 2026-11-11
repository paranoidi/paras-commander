package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/jobs"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

// themeAppSamples are the theme picker's app preview page samples. Each is drawn by the helper the
// real screen uses (drawMenuBar and drawPulldownMenu, drawJobsListJobRow, drawJobsProgressBar),
// fed hard-coded data, so the previewed theme styles them exactly as it would the app.
func themeAppSamples(model Model) dialog.ThemeAppSamples {
	return dialog.ThemeAppSamples{
		MenuBar:  drawThemeSampleMenuBar,
		Jobs:     drawThemeSampleJobs,
		Progress: drawThemeSampleProgress,
		Panel: func(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme) {
			drawThemeSamplePanel(screen, x, y, width, maxY, styles, model)
		},
	}
}

// themeSampleJobStatuses lists every job status, in the order of the theme's [status] section.
var themeSampleJobStatuses = []jobs.Status{
	jobs.StatusScanning, jobs.StatusQueued, jobs.StatusRunning, jobs.StatusPaused,
	jobs.StatusWaitingDecision, jobs.StatusFailed, jobs.StatusCanceling, jobs.StatusCanceled,
	jobs.StatusCompleted,
}

// themeSampleNow is the fixed clock the sample job rows are drawn at, so they render the same every time.
var themeSampleNow = time.Date(2026, time.January, 2, 15, 4, 5, 0, time.UTC)

// themeSampleJobWidth is the width of a job row up to its progress bar (see jobsListCol*).
const themeSampleJobWidth = jobsListColPrefix + jobsListColStatus + jobsListColETA + jobsListColSpeed

// themeSampleProgressLabelWidth is the label column of the progress samples.
const themeSampleProgressLabelWidth = 11

// drawThemeSampleMenuBar draws a menu bar with two titles, the first pulled down over two entries
// with the first selected.
func drawThemeSampleMenuBar(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme) {
	menus := []menu.Definition{
		{Label: "Example 1", Shortcut: 'E', Items: []menu.Item{
			{Label: "Entry 1", Shortcut: 'E'},
			{Label: "Entry 2", Shortcut: '2'},
		}},
		{Label: "Example 2", Shortcut: '2'},
	}
	state := menu.State{Open: true, ActiveMenu: 0, PulldownOpen: true, SelectedItem: 0}
	bar := Rect{X: x, Y: y, Width: width, Height: 1}
	drawMenuBar(screen, bar, state, MenuHover{}, menus, styles, MenuBarJobsStrip{}, "", "", false, 0, 0)
	// The pulldown stops above maxY like the rest of the pane (Footer.Y is where it must end).
	drawPulldownMenu(screen, Layout{Width: x + width, Menu: bar, Footer: Rect{Y: maxY}}, state, menus, styles)
}

// drawThemeSampleJobs draws one Jobs list row per job status, without the progress column.
func drawThemeSampleJobs(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme) {
	if width < themeSampleJobWidth {
		return
	}
	rowBase := styles.JobsRow.Background(auxPanelContentBG(styles, false)) // as drawJobsView
	for i, status := range themeSampleJobStatuses {
		if y+i >= maxY {
			return
		}
		primitive.Text(screen, x, y+i, width, "", rowBase)
		entry := JobEntry{Type: string(jobs.TypeCopy), Status: string(status), TotalFiles: 10, DoneFiles: 4}
		// The snapshot fields the Queue's ETA and Speed columns read; drawJobsListJobRow shows them
		// only for the statuses the real view does.
		switch status {
		case jobs.StatusRunning:
			entry.QueueETA, entry.DisplaySpeedBPS = "2m 10s", 48<<20
		case jobs.StatusQueued:
			entry.QueueETA = "5m 40s"
		}
		drawJobsListJobRow(screen, x, y+i, x+themeSampleJobWidth, 0, entry, false, rowBase, false, styles, themeSampleNow)
	}
}

// drawThemeSampleProgress draws the progress bar of a running job, of a paused one (the inactive
// palette) and of a completed one.
func drawThemeSampleProgress(screen tcell.Screen, x, y, width, maxY int, styles theme.Theme) {
	label := styles.PanelText.Background(auxPanelContentBG(styles, false))
	bars := []struct {
		label  string
		status jobs.Status
		pct    float64
	}{
		{"Active", jobs.StatusRunning, 45},
		{"Inactive", jobs.StatusPaused, 45},
		{"Completed", jobs.StatusCompleted, 100},
	}
	for i, bar := range bars {
		if y+i >= maxY {
			return
		}
		primitive.Text(screen, x, y+i, themeSampleProgressLabelWidth, bar.label, label)
		fill, track, onFill, onTrack := styles.JobsProgressStyles(string(bar.status))
		drawJobsProgressBar(screen, x+themeSampleProgressLabelWidth, y+i, width-themeSampleProgressLabelWidth, bar.pct, fill, track, onFill, onTrack)
	}
}

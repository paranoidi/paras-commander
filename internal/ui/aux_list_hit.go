package ui

import "time"

// auxListScroll clamps the scroll offset of a one-line-per-entry aux list (Jobs queue, Messages)
// so the last page stays full.
func auxListScroll(scroll, n, visibleRows int) int {
	scroll = max(scroll, 0)
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}
	return scroll
}

// auxListRowAt returns the entry under (x, y) in a one-line-per-entry aux list drawn in rect: a
// header row below the top border, then one entry per row.
func auxListRowAt(rect Rect, scroll, n, x, y int) (int, bool) {
	if x <= rect.X || x >= rect.X+rect.Width-1 {
		return 0, false
	}
	visibleRows := PanelListRows(rect)
	row := y - PanelListTop(rect)
	if row < 0 || row >= visibleRows {
		return 0, false
	}
	idx := auxListScroll(scroll, n, visibleRows) + row
	return idx, idx < n
}

// JobsListEntryAt returns the index of the Jobs queue row (in JobsViewState.JobsListRows: a job
// or a source sub-row) drawn at (x, y). There is none while the Operations pane is maximized
// over the view.
func JobsListEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	st := model.JobsView
	var sel JobEntry
	if st.Selected >= 0 && st.Selected < len(model.JobsList) {
		sel = model.JobsList[st.Selected]
	}
	rects := JobsViewLayout(layout, model.JobsOpsBottom, sel, time.Now(), model.JobsThroughputChartEnabled)
	if _, maximized := jobsMaximizedPane(st, sel, rects); maximized {
		return 0, false
	}
	return auxListRowAt(rects.List, st.ListScroll, len(st.JobsListRows(model.JobsList)), x, y)
}

// MessagesListEntryAt returns the logical message drawn at (x, y): a continuation row (no time)
// belongs to the message above it.
func MessagesListEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	rect := MergeTwinPanelRects(layout.Primary, layout.Secondary, model.SplitOrientation)
	log := model.MessageLog
	idx, ok := auxListRowAt(rect, model.MessagesView.ListScroll, len(log), x, y)
	if !ok {
		return 0, false
	}
	for idx > 0 && log[idx].Time == "" {
		idx--
	}
	return idx, true
}

// commandsListRow is one drawn line of the Commands list: an entry's command line (Macro -1) or
// one of its macro lines.
type commandsListRow struct {
	Y     int
	Entry int // index into the entries slice
	Macro int
}

// commandsListRows lays out the Commands list in rect, newest first from the row below the top
// border, each entry followed by its macro lines, up to the panel's content rows.
func commandsListRows(rect Rect, state CommandsViewState, entries []CommandRunEntry) []commandsListRow {
	visibleRows := JobsPanelContentRows(rect)
	n := len(entries)
	if visibleRows <= 0 || n == 0 {
		return nil
	}
	scroll := max(0, min(state.ListScroll, commandsMaxListScroll(entries, visibleRows)))
	rows := make([]commandsListRow, 0, visibleRows)
	y := rect.Y + 1
	bottom := y + visibleRows
	for row := scroll; row < n && y < bottom; row++ {
		idx := CommandsDisplayIndex(n, row)
		rows = append(rows, commandsListRow{Y: y, Entry: idx, Macro: -1})
		y++
		for mi := range entries[idx].Macros {
			if y >= bottom {
				break
			}
			rows = append(rows, commandsListRow{Y: y, Entry: idx, Macro: mi})
			y++
		}
	}
	return rows
}

// CommandsListEntryAt returns the Commands entry drawn at (x, y), on its command or macro lines.
// It reads CommandsDisplay, the render-time snapshot of what is on screen: CommandsList itself
// belongs to the worker goroutines.
func CommandsListEntryAt(layout Layout, model *Model, x, y int) (int, bool) {
	rect := layout.Primary
	if x <= rect.X || x >= rect.X+rect.Width-1 {
		return 0, false
	}
	for _, row := range commandsListRows(rect, model.CommandsView, model.CommandsDisplay) {
		if row.Y == y {
			return row.Entry, true
		}
	}
	return 0, false
}

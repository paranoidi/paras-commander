package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/gitstatus"
	"github.com/paranoidi/paras-commander/internal/localfs"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const (
	panelListGitCells = 2 // staged + unstaged status (eza --git)
	panelListGitGap   = 1 // space before devicon strip
)

// panelListGitStripWidth is horizontal space reserved before the icon strip and name columns.
func panelListGitStripWidth() int {
	return panelListGitCells + panelListGitGap
}

func panelListReservedBeforeName(showGit, nameOnly bool) int {
	if nameOnly || !showGit {
		return 0
	}
	return panelListGitStripWidth()
}

func panelListGitColumnActive(state panel.State, nameOnly bool) bool {
	return state.GitColumnActive && !state.GitPending && !nameOnly
}

func panelGitCell(entry localfs.Entry, byPath map[string]gitstatus.Cell) gitstatus.Cell {
	if byPath == nil {
		return gitstatus.Cell{Staged: gitstatus.NotModified, Unstaged: gitstatus.NotModified}
	}
	if c, ok := byPath[entry.Path]; ok {
		return c
	}
	return gitstatus.Cell{Staged: gitstatus.NotModified, Unstaged: gitstatus.NotModified}
}

func gitStatusDimOnUsageBar(st gitstatus.Status) bool {
	return st == gitstatus.NotModified || st == gitstatus.Ignored
}

func gitStatusCellStyle(st gitstatus.Status, rowStyle tcell.Style, styles theme.Theme, cursorRow, diskUsageOverlay bool, usageAccent tcell.Style) tcell.Style {
	_, rowBG, rowAttrs := rowStyle.Decompose()
	if cursorRow {
		fg, _, _ := rowStyle.Decompose()
		return tcell.StyleDefault.Foreground(fg).Background(rowBG).Attributes(rowAttrs)
	}
	if diskUsageOverlay && gitStatusDimOnUsageBar(st) {
		fg, _, attrs := usageAccent.Decompose()
		return tcell.StyleDefault.Foreground(fg).Background(rowBG).Attributes(attrs)
	}
	fg, _, attrs := styles.PanelGitStyle(st.ThemeKey()).Decompose()
	return tcell.StyleDefault.Foreground(fg).Background(rowBG).Attributes(attrs)
}

func paintGitColumn(screen tcell.Screen, x, y int, cell gitstatus.Cell, rowStyle tcell.Style, styles theme.Theme, cursorRow, diskUsageOverlay bool, usageAccent tcell.Style) {
	for i, st := range []gitstatus.Status{cell.Staged, cell.Unstaged} {
		s := gitStatusCellStyle(st, rowStyle, styles, cursorRow, diskUsageOverlay, usageAccent)
		screen.SetContent(x+i, y, st.Rune(), nil, s)
	}
}

func paintGitGap(screen tcell.Screen, x, y int, style tcell.Style) {
	screen.SetContent(x+panelListGitCells, y, ' ', nil, style)
}

func paintGitHeader(screen tcell.Screen, x, y int, headerStyle tcell.Style, styles theme.Theme) {
	primitive.Text(screen, x, y, panelListGitCells, styles.SymbolGit(), headerStyle)
	paintGitGap(screen, x, y, headerStyle)
}

// paintGitStripBlank fills the git column and gap with spaces (empty listing rows).
func paintGitStripBlank(screen tcell.Screen, x, y int, style tcell.Style) {
	for i := 0; i < panelListGitStripWidth(); i++ {
		screen.SetContent(x+i, y, ' ', nil, style)
	}
}

// paintGitRowTrailingGap paints the separator after status glyphs on data rows.
func paintGitRowTrailingGap(screen tcell.Screen, x, y int, style tcell.Style) {
	paintGitGap(screen, x, y, style)
}

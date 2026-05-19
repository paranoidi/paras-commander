package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
)

const commandsListColMarker = 3
const commandsListColCmd = 28

func drawCommandsView(
	screen tcell.Screen,
	layout Layout,
	state CommandsViewState,
	entries []CommandRunEntry,
	styles theme.Theme,
	chromeBlocked bool,
	userHomeDir string,
) {
	drawCommandsListPanel(screen, layout.Left, state, entries, styles, chromeBlocked, userHomeDir)
	var sel CommandRunEntry
	if state.Selected >= 0 && state.Selected < len(entries) {
		sel = entries[state.Selected]
	}
	stdoutLines := commandPanelLines(sel.Stdout, layout.Right.Width-4)
	stderrLines := commandPanelLines(sel.Stderr, layout.Right.Width-4)
	stderrLineBudget := max(8, min(len(stderrLines)+2, 24))
	stdoutRect, stderrRect := SplitJobsRightColumnFlexTop(layout.Right, stderrLineBudget)
	stdoutFocused := state.FocusPane == 1
	stderrFocused := state.FocusPane == 2
	drawCommandsStreamPanel(screen, stdoutRect, " Stdout ", state.StdoutScroll, stdoutLines, styles, chromeBlocked, stdoutFocused)
	drawCommandsStreamPanel(screen, stderrRect, " Stderr ", state.StderrScroll, stderrLines, styles, chromeBlocked, stderrFocused)
}

func drawCommandsListPanel(screen tcell.Screen, rect Rect, state CommandsViewState, entries []CommandRunEntry, styles theme.Theme, chromeBlocked bool, userHomeDir string) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	active := state.FocusPane == 0
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if active {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if active {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, " Commands ", titleStyle)

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	visibleRows := PanelListRows(rect)
	if visibleRows <= 0 || len(entries) == 0 {
		if visibleRows > 0 {
			emptyStyle := styles.JobsRow.Background(bg)
			primitive.Text(screen, contentX, rect.Y+2, contentW, " No runs ", emptyStyle)
		}
		return
	}

	cmdHdrW := commandsListColCmd - commandsListColMarker
	hdr := fmt.Sprintf("%-*s%-*s%s",
		commandsListColMarker, "",
		cmdHdrW, "Command",
		"Target")
	headerStyle := styles.PanelActiveHeader.Background(bg)
	if chromeBlocked {
		headerStyle = styles.PanelBlockedHeader
	} else if !active {
		headerStyle = styles.PanelInactiveHeader.Background(bg)
	}
	primitive.Text(screen, contentX, rect.Y+1, contentW, hdr, headerStyle)

	n := len(entries)
	scroll := state.ListScroll
	if scroll < 0 {
		scroll = 0
	}
	if scroll > 0 && scroll >= n {
		scroll = max(0, n-visibleRows)
	}
	if scroll+visibleRows > n {
		scroll = max(0, n-visibleRows)
	}

	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		entry := entries[idx]
		lineStyle := styles.JobsRow.Background(bg)
		if idx == state.Selected {
			if chromeBlocked {
				lineStyle = styles.PanelBlockedCursor
			} else if active {
				lineStyle = styles.PanelRowSelected.Background(bg)
			} else {
				lineStyle = styles.PanelCursorInactive.Background(bg)
			}
		}

		markStyle := commandPhaseStyle(entry, styles).Background(bg)
		mark := commandPhaseMark(entry)
		primitive.Text(screen, contentX, y, commandsListColMarker, mark, markStyle)

		cmdColW := commandsListColCmd - commandsListColMarker
		if cmdColW < 1 {
			cmdColW = 1
		}
		cmdShown := truncateRunes(entry.UserCommandLine, cmdColW-1)
		xCmd := contentX + commandsListColMarker
		primitive.Text(screen, xCmd, y, cmdColW, cmdShown, lineStyle)

		tpath := entry.TargetPath
		if userHomeDir != "" && strings.HasPrefix(tpath, userHomeDir+string(filepath.Separator)) {
			tpath = "~" + tpath[len(userHomeDir):]
		}
		targetW := contentW - commandsListColCmd
		if targetW < 1 {
			targetW = 1
		}
		xTarget := contentX + commandsListColCmd
		targetShown := primitive.FitPathForWidth(tpath, targetW)
		primitive.Text(screen, xTarget, y, targetW, targetShown, lineStyle)
	}
}

// Status icons match AGENTS.md (Nerd Font / PUA codepoints).
// Input-required glyph from AGENTS.md is "\U000f02d7" (cyan) when a phase waits on user input.
const (
	iconStatusOngoing = "\U0000f144" //  ongoing (green when running)
	iconStatusPaused  = "\U0000f28b" //  paused (yellow)
	iconStatusStopped = "\U0000f28d" //  stopped (red)
	iconStatusError   = "\U0000f06a" //  error (red)
)

func commandPhaseMark(e CommandRunEntry) string {
	switch e.Phase {
	case CommandRunPending:
		return iconStatusPaused + " "
	case CommandRunRunning:
		return iconStatusOngoing + " "
	case CommandRunDone:
		if e.ExitCode == 0 {
			return iconStatusStopped + " "
		}
		if e.ExitCode == -1 && e.ErrorMsg == "Canceled" {
			return iconStatusStopped + " "
		}
		return iconStatusError + " "
	default:
		return iconStatusError + " "
	}
}

func commandPhaseStyle(e CommandRunEntry, styles theme.Theme) tcell.Style {
	switch e.Phase {
	case CommandRunPending:
		return styles.JobsRow
	case CommandRunRunning:
		return styles.JobsRunning
	case CommandRunDone:
		if e.ExitCode == 0 {
			return styles.JobsDone
		}
		return styles.JobsFailed
	default:
		return styles.JobsRow
	}
}

func commandPanelLines(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	t := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(t) == "" {
		return []string{" (empty)"}
	}
	var out []string
	for _, line := range strings.Split(t, "\n") {
		out = append(out, fitCommandOutputLine(line, width))
	}
	return out
}

func fitCommandOutputLine(line string, width int) string {
	if utf8.RuneCountInString(line) <= width {
		return line
	}
	return primitive.TruncateRight(line, width)
}

func drawCommandsStreamPanel(screen tcell.Screen, rect Rect, title string, scroll int, lines []string, styles theme.Theme, chromeBlocked bool, focused bool) {
	_, bg, _ := styles.PanelActiveSurface.Decompose()
	if chromeBlocked {
		_, bg, _ = styles.PanelBlockedSurface.Decompose()
	}
	active := focused
	var titleStyle tcell.Style
	var borderStyle tcell.Style
	if chromeBlocked {
		borderStyle = styles.PanelBlockedFrame
		titleStyle = styles.PanelBlockedTitle
	} else if active {
		borderStyle = styles.PanelActiveFrame
		titleStyle = styles.PanelActiveTitle
	} else {
		borderStyle = styles.PanelInactiveFrame
		titleStyle = styles.PanelInactiveTitle
	}
	primitive.Box(screen, primitive.Rect(rect), borderStyle)
	inner := primitive.Rect{X: rect.X + 1, Y: rect.Y + 1, Width: rect.Width - 2, Height: rect.Height - 2}
	if inner.Width > 0 && inner.Height > 0 {
		var surface tcell.Style
		if chromeBlocked {
			surface = styles.PanelBlockedSurface
		} else if active {
			surface = styles.PanelActiveSurface
		} else {
			surface = styles.PanelInactiveSurface
		}
		primitive.Fill(screen, inner, ' ', surface)
	}
	titleX := rect.X + 2
	titleWidth := rect.Width - 4
	primitive.TextOverlay(screen, titleX, rect.Y, titleWidth, title, titleStyle)

	body := styles.PanelRowNormal.Background(bg)
	if chromeBlocked {
		body = styles.PanelBlockedRowNormal
	}
	contentTop := rect.Y + 1
	contentH := JobsPanelContentRows(rect)
	if contentH <= 0 {
		return
	}

	if scroll < 0 {
		scroll = 0
	}
	maxStart := max(0, len(lines)-contentH)
	if scroll > maxStart {
		scroll = maxStart
	}

	textX := rect.X + 2
	textW := rect.Width - 4
	if textW < 1 {
		textW = 1
	}
	for i := 0; i < contentH; i++ {
		line := ""
		if i+scroll < len(lines) {
			line = lines[i+scroll]
		}
		primitive.Text(screen, textX, contentTop+i, textW, line, body)
	}
}

package ui

import (
	"fmt"
	"path/filepath"
	"strings"

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
	drawCommandsListPanel(screen, layout.Primary, state, entries, styles, chromeBlocked, userHomeDir)
	var sel CommandRunEntry
	if state.Selected >= 0 && state.Selected < len(entries) {
		sel = entries[state.Selected]
	}
	stdoutRect, stderrRect, stdoutLines, stderrLines := CommandsStreamPanels(layout.Secondary, sel)
	stdoutFocused := state.FocusPane == 1
	stderrFocused := state.FocusPane == 2
	drawCommandsStreamPanel(screen, stdoutRect, " Stdout ", state.StdoutScroll, stdoutLines, styles, chromeBlocked, stdoutFocused)
	drawCommandsStreamPanel(screen, stderrRect, " Stderr ", state.StderrScroll, stderrLines, styles, chromeBlocked, stderrFocused)
}

// CommandsStreamPanels returns stdout/stderr panel geometry and wrapped output lines for one run entry.
func CommandsStreamPanels(column Rect, entry CommandRunEntry) (stdoutRect, stderrRect Rect, stdoutLines, stderrLines []string) {
	textW := column.Width - 4
	if textW < 1 {
		textW = 1
	}
	stdoutLines = CommandPanelLines(entry.Stdout, textW)
	stderrLines = CommandPanelLines(CommandStderrDisplay(entry), textW)
	stderrLineBudget := max(8, min(len(stderrLines)+2, 24))
	stdoutRect, stderrRect = SplitJobsSecondaryColumnFlexTop(column, stderrLineBudget)
	return stdoutRect, stderrRect, stdoutLines, stderrLines
}

func drawCommandsListPanel(screen tcell.Screen, rect Rect, state CommandsViewState, entries []CommandRunEntry, styles theme.Theme, chromeBlocked bool, userHomeDir string) {
	active := state.FocusPane == 0
	layout := drawAuxPanelChrome(screen, rect, " Commands ", "", active, chromeBlocked, styles)
	bg := layout.ContentBG

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

	rowBase := styles.JobsRow.Background(bg)
	for row := 0; row < visibleRows; row++ {
		idx := scroll + row
		y := rect.Y + 2 + row
		if idx >= n {
			break
		}
		entry := entries[idx]
		selected := idx == state.Selected
		lineStyle := auxPanelListRowStyle(styles, rowBase, selected, chromeBlocked, active)
		_, lineBG, _ := lineStyle.Decompose()
		paintAuxPanelRowMargins(screen, rect, contentX, contentW, y, lineStyle)

		markStyle := commandPhaseStyle(entry, styles).Background(lineBG)
		if selected {
			markStyle = lineStyle
		}
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

// CommandPanelLines splits command stdout/stderr into wrapped display lines for the Commands view.
func CommandPanelLines(text string, width int) []string {
	if width < 1 {
		width = 1
	}
	t := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(t) == "" {
		return []string{" (empty)"}
	}
	return WrapTextLines(t, width)
}

func drawCommandsStreamPanel(screen tcell.Screen, rect Rect, title string, scroll int, lines []string, styles theme.Theme, chromeBlocked bool, focused bool) {
	layout := drawAuxPanelChrome(screen, rect, title, "", focused, chromeBlocked, styles)
	body := auxPanelBodyText(styles, chromeBlocked, layout.ContentBG)
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

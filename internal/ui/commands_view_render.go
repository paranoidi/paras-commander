package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
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
	stdoutRect, stderrRect, stdoutLines, stderrLines := CommandsStreamPanels(layout.Secondary, sel, auxPanelBodyText(styles, chromeBlocked, auxPanelContentBG(styles, chromeBlocked)))
	stdoutFocused := state.FocusPane == 1
	stderrFocused := state.FocusPane == 2
	drawCommandsStreamPanel(screen, stdoutRect, " Stdout ", StreamScroll(state.StdoutScroll, state.StdoutDetached), stdoutLines, styles, chromeBlocked, stdoutFocused)
	drawCommandsStreamPanel(screen, stderrRect, " Stderr ", StreamScroll(state.StderrScroll, state.StderrDetached), stderrLines, styles, chromeBlocked, stderrFocused)
}

type streamWrapKey struct {
	// Full strings, not lengths: capped output keeps a constant length while its content
	// moves. Comparing is cheap (same backing pointer short-circuits; changes differ early).
	id, stdout, stderr, errMsg string
	textW                      int
}

type streamWrapLines struct{ stdout, stderr [][]AnsiCell }

// streamWrapCache holds the wrapped lines of the last-rendered entry, one copy per base style
// (the renderer and the key handler pass different bases). ponytail: single entry; a running
// row re-wraps on every change (at most 10/s), upgrade = wrap only the appended tail.
var streamWrapCache struct {
	mu     sync.Mutex
	key    streamWrapKey
	byBase map[tcell.Style]streamWrapLines
}

func commandStreamLines(entry CommandRunEntry, textW int, base tcell.Style) streamWrapLines {
	key := streamWrapKey{entry.ID, entry.Stdout, entry.Stderr, entry.ErrorMsg, textW}
	c := &streamWrapCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byBase == nil || c.key != key {
		c.key, c.byBase = key, map[tcell.Style]streamWrapLines{}
	}
	if l, ok := c.byBase[base]; ok {
		return l
	}
	l := streamWrapLines{
		stdout: CommandPanelLines(entry.Stdout, textW, base),
		stderr: CommandPanelLines(CommandStderrDisplay(entry), textW, base),
	}
	c.byBase[base] = l
	return l
}

// CommandsStreamPanels returns stdout/stderr panel geometry and wrapped output lines for one run entry.
// The returned slices are shared with the wrap cache and must not be mutated.
func CommandsStreamPanels(column Rect, entry CommandRunEntry, base tcell.Style) (stdoutRect, stderrRect Rect, stdoutLines, stderrLines [][]AnsiCell) {
	textW := column.Width - 4
	if textW < 1 {
		textW = 1
	}
	l := commandStreamLines(entry, textW, base)
	stdoutLines, stderrLines = l.stdout, l.stderr
	stderrLineBudget := max(8, min(len(stderrLines)+2, 24))
	stdoutRect, stderrRect = SplitJobsSecondaryColumnFlexTop(column, stderrLineBudget)
	return stdoutRect, stderrRect, stdoutLines, stderrLines
}

// CommandsStreamMaxScroll returns the largest valid scroll offset of the stdout and stderr
// panes for entry (the single source for key handling).
func CommandsStreamMaxScroll(column Rect, entry CommandRunEntry) (stdoutMax, stderrMax int) {
	stdoutRect, stderrRect, stdoutLines, stderrLines := CommandsStreamPanels(column, entry, tcell.StyleDefault)
	return max(0, len(stdoutLines)-JobsPanelContentRows(stdoutRect)),
		max(0, len(stderrLines)-JobsPanelContentRows(stderrRect))
}

func drawCommandsListPanel(screen tcell.Screen, rect Rect, state CommandsViewState, entries []CommandRunEntry, styles theme.Theme, chromeBlocked bool, userHomeDir string) {
	active := state.FocusPane == 0
	layout := drawAuxPanelChrome(screen, rect, " Commands ", "", active, chromeBlocked, false, styles)
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
		cmdShown := primitive.TruncateRight(entry.UserCommandLine, cmdColW-1)
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
// Input-required icon from AGENTS.md is "\U000f02d7" (cyan) when a phase waits on user input.
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
func CommandPanelLines(text string, width int, base tcell.Style) [][]AnsiCell {
	if width < 1 {
		width = 1
	}
	t := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(t) == "" {
		return [][]AnsiCell{AnsiStyledCells(" (empty)", base)}
	}
	return WrapAnsiCells(AnsiStyledCells(t, base), width)
}

func drawCommandsStreamPanel(screen tcell.Screen, rect Rect, title string, scroll int, lines [][]AnsiCell, styles theme.Theme, chromeBlocked bool, focused bool) {
	layout := drawAuxPanelChrome(screen, rect, title, "", focused, chromeBlocked, false, styles)
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
		var line []AnsiCell
		if i+scroll < len(lines) {
			line = lines[i+scroll]
		}
		previewpanel.DrawCells(screen, textX, contentTop+i, textW, line, body)
	}
}

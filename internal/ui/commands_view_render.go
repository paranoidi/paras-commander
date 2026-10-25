package ui

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/cmdmacro"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/previewpanel"
)

const commandsListColMarker = 3

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
	stdoutRect, stderrRect, stdoutLines, stderrLines := CommandsStreamPanels(layout.Secondary, sel, auxPanelBodyText(styles, chromeBlocked, auxPanelContentBG(styles, chromeBlocked)), styles)
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

type streamWrapVariant struct {
	base, empty tcell.Style
	emptyText   string
}

type streamWrapLines struct{ stdout, stderr [][]AnsiCell }

// streamWrapCache holds the wrapped lines of the last-rendered entry, one copy per base/empty-text variant
// (the renderer and the key handler pass different bases). ponytail: single entry; a running
// row re-wraps on every change (at most 10/s), upgrade = wrap only the appended tail.
var streamWrapCache struct {
	mu     sync.Mutex
	key    streamWrapKey
	byBase map[streamWrapVariant]streamWrapLines
}

func commandStreamLines(entry CommandRunEntry, textW int, base tcell.Style, styles theme.Theme) streamWrapLines {
	key := streamWrapKey{entry.ID, entry.Stdout, entry.Stderr, entry.ErrorMsg, textW}
	c := &streamWrapCache
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byBase == nil || c.key != key {
		c.key, c.byBase = key, map[streamWrapVariant]streamWrapLines{}
	}
	variant := streamWrapVariant{base, styles.PanelEmptyStyle(), styles.EmptyText()}
	if l, ok := c.byBase[variant]; ok {
		return l
	}
	l := streamWrapLines{
		stdout: CommandPanelLines(entry.Stdout, textW, base, styles),
		stderr: CommandPanelLines(CommandStderrDisplay(entry), textW, base, styles),
	}
	c.byBase[variant] = l
	return l
}

// CommandsStreamPanels returns stdout/stderr panel geometry and wrapped output lines for one run entry.
// The returned slices are shared with the wrap cache and must not be mutated.
func CommandsStreamPanels(column Rect, entry CommandRunEntry, base tcell.Style, styles theme.Theme) (stdoutRect, stderrRect Rect, stdoutLines, stderrLines [][]AnsiCell) {
	textW := column.Width - 4
	if textW < 1 {
		textW = 1
	}
	l := commandStreamLines(entry, textW, base, styles)
	stdoutLines, stderrLines = l.stdout, l.stderr
	stderrLineBudget := max(8, min(len(stderrLines)+2, 24))
	stdoutRect, stderrRect = SplitJobsSecondaryColumnFlexTop(column, stderrLineBudget)
	return stdoutRect, stderrRect, stdoutLines, stderrLines
}

// CommandsStreamMaxScroll returns the largest valid scroll offset of the stdout and stderr
// panes for entry (the single source for key handling).
func CommandsStreamMaxScroll(column Rect, entry CommandRunEntry) (stdoutMax, stderrMax int) {
	stdoutRect, stderrRect, stdoutLines, stderrLines := CommandsStreamPanels(column, entry, tcell.StyleDefault, theme.Theme{})
	return max(0, len(stdoutLines)-JobsPanelContentRows(stdoutRect)),
		max(0, len(stderrLines)-JobsPanelContentRows(stderrRect))
}

func drawCommandsListPanel(screen tcell.Screen, rect Rect, state CommandsViewState, entries []CommandRunEntry, styles theme.Theme, chromeBlocked bool, userHomeDir string) {
	active := state.FocusPane == 0
	filterUI := active && state.Filter.UIActive()
	title := " Commands "
	if filterUI {
		title = "" // drawQuickFilterTitle owns the top row
	}
	layout := drawAuxPanelChrome(screen, rect, title, "", active, chromeBlocked, false, styles)
	if filterUI {
		drawQuickFilterTitle(screen, layout.TitleX, rect.Y, layout.TitleWidth, state.Filter, styles)
	}
	bg := layout.ContentBG

	contentX := rect.X + 2
	contentW := rect.Width - 4
	if contentW < 1 {
		contentW = 1
	}

	visibleRows := JobsPanelContentRows(rect) // no header row: the list starts below the border
	if visibleRows <= 0 || len(entries) == 0 {
		if visibleRows > 1 { // second content row, matching the Jobs list
			primitive.Text(screen, contentX, rect.Y+2, contentW, styles.EmptyText(), styles.PanelEmptyStyle().Background(bg))
		}
		return
	}

	cmdIcon := styles.IconCommandsCommand() + " "
	xCmd := contentX + commandsListColMarker
	cmdW := max(1, contentW-commandsListColMarker)

	rowBase := styles.JobsRow.Background(bg)
	var segs []string
	var segRanges [][]search.Range
	for _, row := range commandsListRows(rect, state, entries) {
		idx, y := row.Entry, row.Y
		entry := entries[idx]
		selected := idx == state.Selected
		lineStyle := auxPanelListRowStyle(styles, rowBase, selected, chromeBlocked, active)
		_, lineBG, _ := lineStyle.Decompose()
		matchStyle := styles.FuzzyHighlight
		if selected && active {
			matchStyle = styles.FuzzyHighlightCursor
		}
		matchStyle = matchStyle.Background(lineBG)
		paintAuxPanelRowMargins(screen, rect, contentX, contentW, y, lineStyle)

		if row.Macro < 0 {
			markStyle := commandPhaseStyle(entry, styles).Background(lineBG)
			if selected {
				markStyle = lineStyle
			}
			primitive.Text(screen, contentX, y, commandsListColMarker, commandPhaseMark(entry), markStyle)
			segs = CommandsFilterSegments(entry, userHomeDir)
			segRanges = splitSegmentRanges(segs, state.Filter.Ranges(idx))
			cmdFull := cmdIcon + segs[0]
			cmdShown := primitive.TruncateRight(cmdFull, cmdW-1)
			var spans []primitive.Span
			if cmdShown == cmdFull { // ponytail: no highlight on a truncated line, like Dedup
				spans = shiftSpans(segRanges[0], utf8.RuneCountInString(cmdIcon), matchStyle)
			}
			primitive.StyledText(screen, xCmd, y, cmdW, cmdShown, lineStyle, spans)
			continue
		}

		mi := row.Macro
		m := entry.Macros[mi]
		primitive.Text(screen, contentX, y, commandsListColMarker, "", lineStyle)
		line := commandMacroLine(m, userHomeDir, cmdW)
		var spans []primitive.Span
		if prefix := commandMacroPrefix(m); strings.HasPrefix(line, prefix+segs[mi+1]) {
			spans = shiftSpans(segRanges[mi+1], utf8.RuneCountInString(prefix), matchStyle)
		}
		primitive.StyledText(screen, xCmd, y, cmdW, line, lineStyle, spans)
	}
}

// commandMacroLine renders one macro row line, "%f ~/path", fitting width; a multi-value
// macro (%t/%T) shows its first value plus " (+N more)".
func commandMacroLine(m cmdmacro.MacroUse, userHomeDir string, width int) string {
	prefix := commandMacroPrefix(m)
	if len(m.Values) == 0 {
		return prefix
	}
	v := commandMacroValue(m, userHomeDir)
	suffix := ""
	if len(m.Values) > 1 {
		suffix = fmt.Sprintf(" (+%d more)", len(m.Values)-1)
	}
	pathW := max(1, width-1-runewidth.StringWidth(prefix+suffix))
	return prefix + primitive.FitPathForWidth(v, pathW) + suffix
}

func commandMacroPrefix(m cmdmacro.MacroUse) string { return "%" + string(m.Letter) + " " }

// commandMacroValue is the first value of m as displayed, with the home directory as "~".
func commandMacroValue(m cmdmacro.MacroUse, userHomeDir string) string {
	if len(m.Values) == 0 {
		return ""
	}
	v := m.Values[0]
	if userHomeDir != "" && strings.HasPrefix(v, userHomeDir+string(filepath.Separator)) {
		v = "~" + v[len(userHomeDir):]
	}
	return v
}

// CommandsFilterSegments returns the text the Commands quick filter matches for e: the
// displayed command line, then each macro's displayed first value. Joined with " " it is the
// match label (CommandsFilterLabel); the renderer splits match ranges back per segment.
func CommandsFilterSegments(e CommandRunEntry, userHomeDir string) []string {
	segs := make([]string, 0, 1+len(e.Macros))
	segs = append(segs, commandDisplayLine(e.UserCommandLine))
	for _, m := range e.Macros {
		segs = append(segs, commandMacroValue(m, userHomeDir))
	}
	return segs
}

// CommandsFilterLabel is the single string the quick filter ranks for e.
func CommandsFilterLabel(e CommandRunEntry, userHomeDir string) string {
	return strings.Join(CommandsFilterSegments(e, userHomeDir), " ")
}

// splitSegmentRanges maps rune ranges over the " "-joined segments back to per-segment ranges.
func splitSegmentRanges(segs []string, ranges []search.Range) [][]search.Range {
	out := make([][]search.Range, len(segs))
	off := 0
	for i, seg := range segs {
		n := utf8.RuneCountInString(seg)
		for _, r := range ranges {
			if s, e := max(r.Start, off), min(r.End, off+n); s < e {
				out[i] = append(out[i], search.Range{Start: s - off, End: e - off})
			}
		}
		off += n + 1
	}
	return out
}

func shiftSpans(ranges []search.Range, off int, style tcell.Style) []primitive.Span {
	var spans []primitive.Span
	for _, r := range ranges {
		spans = append(spans, primitive.Span{Start: off + r.Start, End: off + r.End, Style: style})
	}
	return spans
}

// commandDisplayWrappers are leading words dropped (with their flags) from Commands-row
// command lines: shells that only obscure the command they run.
var commandDisplayWrappers = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "ksh": true,
	"csh": true, "tcsh": true, "nu": true,
}

// commandDisplayLine drops a leading shell word and its flags (e.g. "bash -ci 'ls'" → "'ls'")
// from a Commands-row command line.
func commandDisplayLine(line string) string {
	word, rest, _ := strings.Cut(strings.TrimLeft(line, " "), " ")
	if !commandDisplayWrappers[word] {
		return line
	}
	for {
		rest = strings.TrimLeft(rest, " ")
		if !strings.HasPrefix(rest, "-") {
			break
		}
		_, rest, _ = strings.Cut(rest, " ")
	}
	if rest == "" { // a bare shell ("fish -l") keeps its own name
		return line
	}
	return rest
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
func CommandPanelLines(text string, width int, base tcell.Style, styles theme.Theme) [][]AnsiCell {
	if width < 1 {
		width = 1
	}
	t := strings.ReplaceAll(text, "\r\n", "\n")
	if strings.TrimSpace(t) == "" {
		_, bg, _ := base.Decompose()
		return [][]AnsiCell{nil, AnsiStyledCells(styles.EmptyText(), styles.PanelEmptyStyle().Background(bg))}
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

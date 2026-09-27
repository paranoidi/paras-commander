package dialog

import (
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"github.com/paranoidi/paras-commander/internal/pathpick"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/internal/draw"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// PathCompletionRows is the maximum number of visible rows in the completion dropdown.
const PathCompletionRows = 5

// PathCompletion is the shared filesystem-completion dropdown state for a path-shaped dialog
// input: FileDialogField (transfer/flatten destination, file-dialog path fields) and
// PathPickerState's query. Start marks the rune offset in the owning field's value where the
// partial segment begins; Accept replaces value[Start:] (the caret is always at the end — see
// pathpick.Suggest).
type PathCompletion struct {
	Start    int
	Items    []pathpick.Candidate
	Selected int
	Scroll   int
	// Open is true while the dropdown is visible.
	Open bool
	// forValue is the value Items was last computed for, so Set can tell a same-value
	// resync (FS refresh) from an actual edit (which resets Selected/Scroll and re-opens).
	forValue string
	// maxNameW is the widest item label (including a directory's trailing "/"), computed once
	// per Set so the dropdown width stays fixed while scrolling without re-measuring per frame.
	maxNameW int
}

// Set updates the candidate list for value, whose partial segment starts at rune offset start.
// Typing a fresh segment auto-opens the dropdown, but landing on an empty segment (caret right
// after a separator, e.g. just after a Tab-accept of a directory) does not — Tab opens it
// there. A single prefix candidate never opens it (ghost text instead). Re-syncing the same value (e.g. after an FS-change refresh) keeps the current
// Open/Selected state instead of resetting it.
func (c *PathCompletion) Set(value string, start int, items []pathpick.Candidate) {
	sameValue := value == c.forValue
	c.forValue = value
	c.Start = start
	c.Items = items
	c.maxNameW = 0
	for _, item := range items {
		c.maxNameW = max(c.maxNameW, runewidth.StringWidth(completionLabel(item)))
	}

	if len(items) == 0 {
		c.Open = false
		c.Selected = 0
		c.Scroll = 0
		return
	}
	_, ghost := c.ghost(value)
	if sameValue {
		c.Selected = ListClampedSelectionDelta(c.Selected, len(items), 0)
		c.clampScroll()
		if ghost {
			c.Open = false
		}
		return
	}
	c.Selected = 0
	c.Scroll = 0
	partialEmpty := start >= len([]rune(value))
	c.Open = !ghost && !partialEmpty
}

// ghost returns the remaining characters of the single candidate's Name after the typed partial
// segment (value[Start:]) when Items holds exactly one candidate with that non-empty partial
// as a prefix — an empty segment suggests nothing until a letter is typed.
// In that case the dropdown is suppressed (see Set) and the suffix is painted as ghost text
// after the caret instead — a single fuzzy (non-prefix) match still shows the one-row dropdown.
func (c *PathCompletion) ghost(value string) (string, bool) {
	if len(c.Items) != 1 {
		return "", false
	}
	runes := []rune(value)
	partial := string(runes[min(max(c.Start, 0), len(runes)):])
	if partial == "" {
		return "", false
	}
	name := c.Items[0].Name
	if !strings.HasPrefix(name, partial) {
		return "", false
	}
	return name[len(partial):], true
}

// GhostSuffix returns the ghost-text suffix (no trailing "/" — Accept adds that separately), or
// "" when there is no single prefix candidate.
func (c *PathCompletion) GhostSuffix(value string) string {
	suffix, _ := c.ghost(value)
	return suffix
}

// Clear drops all completion state (dropdown closed, no candidates).
func (c *PathCompletion) Clear() {
	*c = PathCompletion{}
}

// Move shifts the selection by delta rows, clamped (no wrap), scrolling to keep it visible.
func (c *PathCompletion) Move(delta int) {
	if !c.Open || len(c.Items) == 0 {
		return
	}
	c.Selected = ListClampedSelectionDelta(c.Selected, len(c.Items), delta)
	c.clampScroll()
}

// Cycle advances the selection by one row, wrapping around. Used by Tab.
func (c *PathCompletion) Cycle() {
	if !c.Open || len(c.Items) == 0 {
		return
	}
	c.Selected = (c.Selected + 1) % len(c.Items)
	c.clampScroll()
}

func (c *PathCompletion) clampScroll() {
	c.Scroll = geom.ScrollOffsetEdge(c.Selected, c.Scroll, PathCompletionRows, len(c.Items), 0)
}

func completionLabel(item pathpick.Candidate) string {
	if item.IsDir {
		return item.Name + "/"
	}
	return item.Name
}

// Accept replaces value[Start:] with the selected item's name (plus a trailing "/" for a
// directory) and clears the completion state. Returns the unchanged value and its end when
// there is nothing to accept.
func (c *PathCompletion) Accept(value string) (string, int) {
	runes := []rune(value)
	if len(c.Items) == 0 || c.Selected < 0 || c.Selected >= len(c.Items) {
		return value, len(runes)
	}
	nameRunes := []rune(completionLabel(c.Items[c.Selected]))
	newRunes := append(runes[:min(max(c.Start, 0), len(runes))], nameRunes...)
	c.Clear()
	return string(newRunes), len(newRunes)
}

// HandlePathCompletionKey handles Tab/Up/Down/Enter/Esc for an open or openable completion
// dropdown on a path input's text focus. value/cursor are the owning field's Value/Cursor,
// mutated in place by Accept. handled is true when the key was consumed by the dropdown
// (caller should not fall through to its own Esc/Enter/Tab/Up/Down handling); accepted is true
// when a candidate was inserted into *value (caller should re-sync completion, arm its
// destination-validate timer, etc. — see callers in apphandler/dialog).
func HandlePathCompletionKey(ev *tcell.EventKey, c *PathCompletion, value *string, cursor *int) (handled, accepted bool) {
	if c == nil || value == nil || cursor == nil {
		return false, false
	}
	if len(c.Items) > 0 && ev.Key() == tcell.KeyTab {
		switch {
		case len(c.Items) == 1:
			*value, *cursor = c.Accept(*value)
			return true, true
		case c.Open:
			c.Cycle()
		default:
			c.Open = true
			c.Selected = 0
			c.clampScroll()
		}
		return true, false
	}
	if !c.Open {
		return false, false
	}
	switch ev.Key() {
	case tcell.KeyUp:
		c.Move(-1)
		return true, false
	case tcell.KeyDown:
		c.Move(1)
		return true, false
	case tcell.KeyEnter:
		*value, *cursor = c.Accept(*value)
		return true, true
	case tcell.KeyEsc:
		c.Open = false
		return true, false
	}
	return false, false
}

// drawPathCompletionDropdown paints the completion dropdown below (or, if it would run off the
// bottom of the screen, above) input row y-1, with candidate names aligned under the segment
// being completed. textX is the column of the value's first rune at scroll 0 and scroll is the
// input's horizontal scroll. x/width are clamped to the screen. When there are more items than
// fit in PathCompletionRows, a vertical scrollbar is painted in the box's own rightmost column
// (the box is widened by one column so names are never overlapped). Draw this at the END of the
// owning dialog's Draw function so later dialog content does not overdraw it.
func drawPathCompletionDropdown(screen tcell.Screen, textX, y, scroll int, c PathCompletion, scrollbarStyle uiscrollbar.Style, styles theme.Theme) {
	if !c.Open || len(c.Items) == 0 {
		return
	}
	segX := textX + max(0, c.Start-scroll)
	if scroll > 0 {
		segX++ // left overflow marker cell
	}
	x := segX - 1 // one space of left padding precedes the name column
	screenW, screenH := screen.Size()
	rows := PathCompletionRows
	if len(c.Items) < rows {
		rows = len(c.Items)
	}
	showScrollbar := len(c.Items) > PathCompletionRows

	const sidePad = 2 // one space each side
	scrollbarCol := 0
	if showScrollbar {
		scrollbarCol = 1
	}
	// Width covers every item, not just the visible window, so it stays fixed while scrolling.
	width := c.maxNameW + sidePad + scrollbarCol
	if width > screenW {
		width = screenW
	}
	if width < 1 {
		return
	}
	if x+width > screenW {
		x = screenW - width
	}
	if x < 0 {
		x = 0
	}
	if y+rows > screenH {
		y -= rows + 1 // draw above the input row instead
	}
	if y < 0 {
		y = 0
	}

	itemStyle := styles.DialogCompletionItem
	selStyle := styles.DialogCompletionItemSelected
	matchStyle := styles.DialogCompletionMatch
	matchSelStyle := styles.DialogCompletionMatchSelected

	for i := 0; i < rows; i++ {
		idx := c.Scroll + i
		rowY := y + i
		if idx < 0 || idx >= len(c.Items) {
			continue
		}
		item := c.Items[idx]
		selected := idx == c.Selected
		rowStyle := itemStyle
		rowMatchStyle := matchStyle
		if selected {
			rowStyle = selStyle
			rowMatchStyle = matchSelStyle
		}

		screen.SetContent(x, rowY, ' ', nil, rowStyle) // left padding cell

		name := completionLabel(item)
		nameX := x + 1
		nameW := width - sidePad - scrollbarCol
		spans := make([]primitive.Span, 0, len(item.Ranges))
		for _, r := range item.Ranges {
			spans = append(spans, primitive.Span{Start: r.Start, End: r.End, Style: rowMatchStyle})
		}
		primitive.StyledText(screen, nameX, rowY, nameW, name, rowStyle, spans)
		screen.SetContent(nameX+nameW, rowY, ' ', nil, rowStyle) // right padding cell
	}

	if showScrollbar {
		dropRect := draw.Rect{X: x, Y: y, Width: width, Height: rows}
		draw.DrawDialogListScrollbar(screen, dropRect, y, rows, len(c.Items), c.Scroll, scrollbarStyle, itemStyle, styles)
	}
}

package ui

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

const (
	menuBarPermRightMargin = 1
	menuBarPermGap         = 1
	// menuBarSpinnerCells is one braille frame for global activity indication.
	menuBarSpinnerCells = 1
)

// menuBarPermissionTailRuneCount is the right-aligned tail width: permission text plus optional gap and menu-bar spinner.
func menuBarPermissionTailRuneCount(perm string, showMenuBarSpinner bool) int {
	perm = strings.TrimSpace(perm)
	w := utf8.RuneCountInString(perm)
	if showMenuBarSpinner {
		w += menuBarSpinnerCells
		if utf8.RuneCountInString(perm) > 0 {
			w += 1 // gap between permission string and spinner
		}
	}
	return w
}

// menuBarAttentionPadded returns jobs/conflict attention text with single spaces before and
// after the trimmed core so themes can paint a distinct background on the whole segment.
func menuBarAttentionPadded(attention string) (padded string, ok bool) {
	core := strings.TrimSpace(attention)
	if core == "" {
		return "", false
	}
	return " " + core + " ", true
}

// menuBarRightTailRuneCount includes optional jobs-attention text before the permission tail.
func menuBarRightTailRuneCount(attention, perm string, showMenuBarSpinner bool) int {
	padded, ok := menuBarAttentionPadded(attention)
	attW := 0
	if ok {
		attW = utf8.RuneCountInString(padded)
	}
	base := menuBarPermissionTailRuneCount(perm, showMenuBarSpinner)
	if attW == 0 {
		return base
	}
	if base == 0 {
		w := attW
		if showMenuBarSpinner {
			w += menuBarSpinnerCells + 1
		}
		return w
	}
	return attW + 1 + base
}

// statusCommandGap is the blank cell before each status-command segment and on both sides
// of each separator glyph.
const statusCommandGap = " "

// StatusCommandWidth is the column count drawMenuBarStatusCommands paints for texts:
// " t1 · t2 · " (one leading gap, then each text followed by gap, separator, gap). The
// texts are already capped to their max_width by the caller. Zero texts reserve nothing.
func StatusCommandWidth(texts []string) int {
	if len(texts) == 0 {
		return 0
	}
	w := len(statusCommandGap)
	for _, t := range texts {
		w += utf8.RuneCountInString(t) + 2*len(statusCommandGap) + 1
	}
	return w
}

// drawMenuBarStatusCommands paints the status-command segments in the reserved top-left
// rect: texts in menu.status, the separator glyph after each in menu.status.separator.
func drawMenuBarStatusCommands(screen tcell.Screen, rect Rect, texts []string, styles theme.Theme) {
	sep := string(styles.IconMenuStatusSeparator())
	x, end := rect.X, rect.X+rect.Width
	put := func(s string, style tcell.Style) {
		if x < end {
			primitive.Text(screen, x, rect.Y, end-x, s, style)
		}
		x += utf8.RuneCountInString(s)
	}
	put(statusCommandGap, styles.MenuStatus)
	for _, t := range texts {
		put(t+statusCommandGap, styles.MenuStatus)
		put(sep, styles.MenuStatusSeparator)
		put(statusCommandGap, styles.MenuStatus)
	}
}

// drawMenuBarJobsGapWithPinBadge draws the pin-count badge (if any) at the start of the gap
// region, then the jobs queue/progress strip in whatever span remains. Returns lightbarExited
// from the inner DrawMenuBarJobsGap call (see its doc comment).
func drawMenuBarJobsGapWithPinBadge(screen tcell.Screen, y, gapStart, gapWidth int, jobsStrip MenuBarJobsStrip, pinCount int, styles theme.Theme) bool {
	if gapWidth <= 0 {
		return false
	}
	consumed := drawMenuBarPinBadge(screen, y, gapStart, gapWidth, pinCount, styles)
	return DrawMenuBarJobsGap(screen, y, gapStart+consumed, gapWidth-consumed, jobsStrip, styles)
}

// drawMenuBarBlank fills the menu row with menu background and no labels (modal overlays block the menu).
func drawMenuBarBlank(screen tcell.Screen, rect Rect, styles theme.Theme, jobsStrip MenuBarJobsStrip, attention, perm string, showMenuBarSpinner bool, spinPhase uint8, pinCount int) {
	primitive.Text(screen, rect.X, rect.Y, rect.Width, "", styles.MenuBarInactive)
	tailW := menuBarRightTailRuneCount(attention, perm, showMenuBarSpinner)
	clipExclusive := menuBarMenusClipExclusive(rect, tailW)
	gapStart := min(menuBarContentStart(rect), clipExclusive)
	gapWidth := clipExclusive - gapStart
	drawMenuBarJobsGapWithPinBadge(screen, rect.Y, gapStart, gapWidth, jobsStrip, pinCount, styles)
	drawMenuBarRightTail(screen, rect, attention, perm, styles.MenuBarAlert, styles.MenuDetail, styles.MenuSpinner, showMenuBarSpinner, spinPhase)
}

func drawMenuBar(screen tcell.Screen, rect Rect, state menu.State, menus []menu.Definition, styles theme.Theme, jobsStrip MenuBarJobsStrip, attention, perm string, showMenuBarSpinner bool, spinPhase uint8, pinCount int) {
	x := rect.X
	barStyle := styles.MenuBarInactive
	if state.Open {
		barStyle = styles.MenuBarActive
	}
	primitive.Text(screen, x, rect.Y, rect.Width, "", barStyle)
	tailW := menuBarRightTailRuneCount(attention, perm, showMenuBarSpinner)
	clipExclusive := menuBarMenusClipExclusive(rect, tailW)
	spans, x := menuBarLabelSpans(rect, menus, clipExclusive)
	for index, span := range spans {
		style := barStyle
		if state.Open && index == state.ActiveMenu {
			style = styles.MenuBarSelected
		}
		drawMenuBarLabel(screen, span.X, rect.Y, span.Width, " "+menus[index].Label+" ", menus[index].Shortcut, style, styles.MenuBarAccent, state.Open && !state.PulldownOpen)
	}
	gapStart := min(max(x, menuBarContentStart(rect)), clipExclusive)
	gapWidth := clipExclusive - gapStart
	drawMenuBarJobsGapWithPinBadge(screen, rect.Y, gapStart, gapWidth, jobsStrip, pinCount, styles)
	drawMenuBarRightTail(screen, rect, attention, perm, styles.MenuBarAlert, styles.MenuDetail, styles.MenuSpinner, showMenuBarSpinner, spinPhase)
}

// menuBarContentStart is the first column menu-bar content may use: one cell past rect.X
// when status_command text sits to the left (rect.X > 0), so the jobs strip or pin badge
// never abuts it. Menu labels carry their own leading space and start at rect.X.
func menuBarContentStart(rect Rect) int {
	if rect.X > 0 {
		return rect.X + 1
	}
	return rect.X
}

// menuBarMenusClipExclusive is the first column after the menu label area (before permission tail).
func menuBarMenusClipExclusive(rect Rect, permissionTailRunes int) int {
	if permissionTailRunes <= 0 {
		return rect.X + rect.Width
	}
	permStart := rect.X + rect.Width - menuBarPermRightMargin - permissionTailRunes
	return max(rect.X, permStart-menuBarPermGap)
}

// menuBarLabelSpan is the drawn extent of one top menu label on the menu row.
type menuBarLabelSpan struct {
	X, Width int
}

// menuBarLabelSpans lays out the top menu labels from rect.X up to clipExclusive: one span per
// label that fits (the last may be cut short), and the column just past the label area.
func menuBarLabelSpans(rect Rect, menus []menu.Definition, clipExclusive int) (spans []menuBarLabelSpan, endX int) {
	x := rect.X
	for _, menuDefinition := range menus {
		labelW := utf8.RuneCountInString(" " + menuDefinition.Label + " ")
		avail := clipExclusive - x
		if avail <= 0 {
			break
		}
		if labelW > avail {
			spans = append(spans, menuBarLabelSpan{X: x, Width: avail})
			return spans, x + avail
		}
		spans = append(spans, menuBarLabelSpan{X: x, Width: labelW})
		x += labelW + 1
	}
	return spans, x
}

// menuBarMenusEndX returns the column just after menu labels laid out like drawMenuBar (respecting permission clip).
func menuBarMenusEndX(rect Rect, menus []menu.Definition, permissionTailRunes int) int {
	clip := menuBarMenusClipExclusive(rect, permissionTailRunes)
	_, x := menuBarLabelSpans(rect, menus, clip)
	return min(max(x, menuBarContentStart(rect)), clip)
}

// DrawMenuBarJobsGapOnly repaints the jobs queue/progress gap between menu labels and the right tail.
// It does not redraw menu labels, the tail, or the rest of the frame — use after job progress updates.
// Returns painted (whether anything was drawn) and lightbarExited (see DrawMenuBarJobsGap).
func DrawMenuBarJobsGapOnly(screen tcell.Screen, layout Layout, model Model, menus []menu.Definition, styles theme.Theme) (painted, lightbarExited bool) {
	if !model.MenuBarLayoutReserved() || layout.Menu.Width <= 0 {
		return false, false
	}
	showMenuBarSpinner := model.MenuBarActivitySpinner
	tailW := menuBarRightTailRuneCount(model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner)
	rect := layout.Menu
	clipEx := menuBarMenusClipExclusive(rect, tailW)
	var gapStart, gapWidth int
	if model.ModalDialogOpen() {
		gapStart = min(menuBarContentStart(rect), clipEx)
	} else {
		gapStart = menuBarMenusEndX(rect, menus, tailW)
	}
	gapWidth = clipEx - gapStart
	if gapWidth <= 0 {
		return false, false
	}
	exited := drawMenuBarJobsGapWithPinBadge(screen, rect.Y, gapStart, gapWidth, model.MenuBarJobs, len(model.PinnedItems), styles)
	return true, exited
}

// DrawMenuBarPermissionTailOnly repaints the jobs-attention, permission, and spinner tail on the
// menu row (used after file-list navigation updates the active row mode string).
func DrawMenuBarPermissionTailOnly(screen tcell.Screen, layout Layout, model Model, styles theme.Theme) bool {
	if !model.MenuBarLayoutReserved() || layout.Menu.Width <= 0 {
		return false
	}
	showSpinner := model.MenuBarActivitySpinner
	rect := layout.Menu
	tailW := menuBarRightTailRuneCount(model.MenuBarJobsAttention, model.MenuBarPermission, showSpinner)
	if tailW <= 0 {
		return false
	}
	tailStart := rect.X + rect.Width - menuBarPermRightMargin - tailW
	if tailStart < rect.X {
		tailStart = rect.X
	}
	barStyle := styles.MenuBarInactive
	if model.Menu.Open {
		barStyle = styles.MenuBarActive
	}
	primitive.Text(screen, tailStart, rect.Y, rect.X+rect.Width-tailStart, "", barStyle)
	drawMenuBarRightTail(screen, rect, model.MenuBarJobsAttention, model.MenuBarPermission, styles.MenuBarAlert, styles.MenuDetail, styles.MenuSpinner, showSpinner, model.SpinPhase)
	return true
}

// DrawMenuBarSpinnerOnly updates the single activity-spinner cell on the menu row without a full repaint.
func DrawMenuBarSpinnerOnly(screen tcell.Screen, layout Layout, model Model, styles theme.Theme) bool {
	if model.ViewMode == ViewFilePreview {
		return drawFullscreenPreviewSpinner(screen, layout, model, styles)
	}
	if !model.MenuBarLayoutReserved() || layout.Menu.Width <= 0 {
		return false
	}
	rect := layout.Menu
	last, ok := menuBarSpinnerX(rect)
	if !model.MenuBarActivitySpinner || !ok {
		return false
	}
	screen.SetContent(last, rect.Y, MenuBarSpinnerIcon(model.SpinPhase), nil, styles.MenuSpinner)
	return true
}

// menuBarSpinnerX is the activity spinner's column at rect's trailing edge (inside the
// permission-tail right margin); ok is false when rect is too narrow to hold it.
func menuBarSpinnerX(rect Rect) (x int, ok bool) {
	x = rect.X + rect.Width - menuBarPermRightMargin - 1
	return x, x >= rect.X && x < rect.X+rect.Width
}

func drawMenuBarRightTail(screen tcell.Screen, rect Rect, attention, perm string, alertStyle, detailStyle, spinnerStyle tcell.Style, showMenuBarSpinner bool, spinPhase uint8) {
	perm = strings.TrimSpace(perm)
	paddedAtt, attOk := menuBarAttentionPadded(attention)
	permW := utf8.RuneCountInString(perm)
	attRunes := []rune(paddedAtt)
	if rect.Width <= 0 {
		return
	}
	if permW == 0 && !attOk && !showMenuBarSpinner {
		return
	}

	last, spinnerFits := menuBarSpinnerX(rect)
	if showMenuBarSpinner {
		if spinnerFits {
			screen.SetContent(last, rect.Y, MenuBarSpinnerIcon(spinPhase), nil, spinnerStyle)
		}
		last--
		if permW > 0 {
			if last >= rect.X && last < rect.X+rect.Width {
				screen.SetContent(last, rect.Y, ' ', nil, detailStyle)
			}
			last--
		} else if attOk {
			if last >= rect.X && last < rect.X+rect.Width {
				screen.SetContent(last, rect.Y, ' ', nil, detailStyle)
			}
			last--
		}
	}
	startPerm := last - permW + 1
	col := 0
	for _, r := range perm {
		x := startPerm + col
		if x < rect.X || x > last || x >= rect.X+rect.Width {
			break
		}
		screen.SetContent(x, rect.Y, r, nil, detailStyle)
		col++
	}
	if !attOk {
		return
	}
	if permW > 0 {
		gapX := startPerm - 1
		if gapX >= rect.X && gapX < rect.X+rect.Width {
			screen.SetContent(gapX, rect.Y, ' ', nil, detailStyle)
		}
		startAtt := startPerm - 1 - len(attRunes)
		drawRunesClipped(screen, startAtt, rect.Y, attRunes, rect, alertStyle)
		return
	}
	startAtt := last - len(attRunes) + 1
	drawRunesClipped(screen, startAtt, rect.Y, attRunes, rect, alertStyle)
}

// drawRunesClipped paints runes starting at column startX on row y, skipping cells left of
// rect and stopping at the first cell at or past rect's right edge, moved out of
// drawMenuBarRightTail's two identical bounds-checked attention-icon paint loops.
func drawRunesClipped(screen tcell.Screen, startX, y int, runes []rune, rect Rect, style tcell.Style) {
	for i, r := range runes {
		x := startX + i
		if x < rect.X {
			continue
		}
		if x >= rect.X+rect.Width {
			break
		}
		screen.SetContent(x, y, r, nil, style)
	}
}

func drawMenuBarLabel(screen tcell.Screen, x, y, width int, label string, shortcut rune, style, accent tcell.Style, menuOpen bool) {
	if width <= 0 {
		return
	}
	column := 0
	highlighted := false
	for _, r := range label {
		if column >= width {
			break
		}
		nextStyle := style
		if !highlighted && menuOpen && shortcut != 0 && unicode.ToLower(r) == unicode.ToLower(shortcut) {
			nextStyle = dialog.AccentIconStyle(style, accent)
			highlighted = true
		}
		screen.SetContent(x+column, y, r, nil, nextStyle)
		column++
	}
}

// pulldownRect is the frame of the open pulldown, below its top menu label and clamped to the
// screen width and the footer. ok is false when no pulldown is open or it has no items.
func pulldownRect(layout Layout, state menu.State, menus []menu.Definition) (Rect, bool) {
	if !state.PulldownOpen || state.ActiveMenu < 0 || state.ActiveMenu >= len(menus) {
		return Rect{}, false
	}
	menuDefinition := menus[state.ActiveMenu]
	if len(menuDefinition.Items) == 0 {
		return Rect{}, false
	}
	// Unclipped spans: the pulldown stays under its label even when the label is cut off.
	spans, _ := menuBarLabelSpans(layout.Menu, menus, math.MaxInt)
	menuX := spans[state.ActiveMenu].X
	width := pulldownWidth(menuDefinition)
	if menuX+width > layout.Width {
		menuX = max(0, layout.Width-width)
	}
	rect := Rect{X: menuX, Y: layout.Menu.Y + 1, Width: width, Height: len(menuDefinition.Items) + 2}
	if rect.Y+rect.Height > layout.Footer.Y {
		rect.Height = max(2, layout.Footer.Y-rect.Y)
	}
	return rect, true
}

func drawPulldownMenu(screen tcell.Screen, layout Layout, state menu.State, menus []menu.Definition, styles theme.Theme) {
	rect, ok := pulldownRect(layout, state, menus)
	if !ok {
		return
	}
	menuDefinition := menus[state.ActiveMenu]

	primitive.Box(screen, primitive.Rect(rect), styles.MenuDropdownFrame, primitive.SharpBorder)
	itemRows := rect.Height - 2
	for row := 0; row < itemRows && row < len(menuDefinition.Items); row++ {
		item := menuDefinition.Items[row]
		if item.Separator {
			drawMenuSeparator(screen, rect.X, rect.Y+1+row, rect.Width, styles.MenuDropdownFrame)
			continue
		}
		style := styles.MenuDropdown
		if row == state.SelectedItem {
			style = styles.MenuDropdownSelected
		}
		drawMenuItem(screen, rect.X+1, rect.Y+1+row, rect.Width-2, item, style, styles.MenuDropdownAccent)
	}
}

func drawMenuSeparator(screen tcell.Screen, x, y, width int, style tcell.Style) {
	if width <= 1 {
		return
	}
	screen.SetContent(x, y, '├', nil, style)
	for column := 1; column < width-1; column++ {
		screen.SetContent(x+column, y, '─', nil, style)
	}
	screen.SetContent(x+width-1, y, '┤', nil, style)
}

func drawMenuItem(screen tcell.Screen, x, y, width int, item menu.Item, style, accent tcell.Style) {
	if width <= 0 {
		return
	}
	primitive.Text(screen, x, y, width, "", style)
	labelStart := 1
	keyStart := width
	if item.KeyLabel != "" {
		keyStart = max(labelStart, width-utf8.RuneCountInString(item.KeyLabel)-1)
		primitive.Text(screen, x+keyStart, y, width-keyStart, item.KeyLabel, style)
	}
	drawMenuLabel(screen, x+labelStart, y, max(0, keyStart-labelStart-1), item, style, accent)
}

func drawMenuLabel(screen tcell.Screen, x, y, width int, item menu.Item, style, accent tcell.Style) {
	if width <= 0 {
		return
	}
	column := 0
	highlighted := false
	for _, r := range item.Label {
		if column >= width {
			break
		}
		nextStyle := style
		if !highlighted && item.Shortcut != 0 && unicode.ToLower(r) == unicode.ToLower(item.Shortcut) {
			nextStyle = dialog.AccentIconStyle(style, accent)
			highlighted = true
		}
		screen.SetContent(x+column, y, r, nil, nextStyle)
		column++
	}
}

func pulldownWidth(menuDefinition menu.Definition) int {
	width := utf8.RuneCountInString(menuDefinition.Label) + 4
	for _, item := range menuDefinition.Items {
		if item.Separator {
			continue
		}
		itemWidth := utf8.RuneCountInString(item.Label) + 4
		if item.KeyLabel != "" {
			itemWidth += utf8.RuneCountInString(item.KeyLabel) + 2
		}
		width = max(width, itemWidth)
	}
	return width
}

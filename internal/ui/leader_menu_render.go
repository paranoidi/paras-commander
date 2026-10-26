package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

const (
	leaderMenuMacroColumns = 4
	leaderMenuItemColumns  = 3
	leaderMenuLeftMargin   = 5
	leaderMenuMarginRows   = 1 // empty surface row above and below content
	leaderMenuColumnGap    = 2 // blank columns between packed macro columns
	leaderMenuRightMargin  = 5 // blank columns after the rightmost column's longest item
)

func leaderMenuIsGrouped(items []LeaderMenuItem) bool {
	for _, it := range items {
		if it.GroupTitle != "" {
			return true
		}
	}
	return false
}

func leaderMenuMaxContentRows(layout geom.Layout) int {
	if layout.Footer.Height <= 0 {
		return 0
	}
	minY := LeaderMenuMinContentY(layout)
	maxHeight := layout.Footer.Y - minY
	minHeight := 2*leaderMenuMarginRows + 1
	if maxHeight < minHeight {
		return 0
	}
	return maxHeight - 2*leaderMenuMarginRows
}

// leaderMenuClampColumn maps an item's (possibly out-of-range) GroupColumn to a
// valid macro-column index, defaulting to 0.
func leaderMenuClampColumn(col int) int {
	if col < 0 || col >= leaderMenuMacroColumns {
		return 0
	}
	return col
}

func leaderMenuSplitByColumn(items []LeaderMenuItem) [leaderMenuMacroColumns][]LeaderMenuItem {
	var buckets [leaderMenuMacroColumns][]LeaderMenuItem
	col := 0
	for _, it := range items {
		if it.GroupTitle != "" {
			col = leaderMenuClampColumn(it.GroupColumn)
		}
		buckets[col] = append(buckets[col], it)
	}
	return buckets
}

func leaderMenuMacroColumnRows(bucket []LeaderMenuItem) int {
	return len(bucket)
}

// leaderMenuContentRows returns content rows for the visible item prefix.
func leaderMenuContentRows(items []LeaderMenuItem) int {
	if len(items) == 0 {
		return 0
	}
	if leaderMenuIsGrouped(items) {
		buckets := leaderMenuSplitByColumn(items)
		max := 0
		for _, b := range buckets {
			if r := leaderMenuMacroColumnRows(b); r > max {
				max = r
			}
		}
		return max
	}
	rows := 0
	col := 0
	for _, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		col++
		if col >= leaderMenuItemColumns {
			rows++
			col = 0
		}
	}
	if col > 0 {
		rows++
	}
	return rows
}

func trimLeaderMenuTrailingGroupHeaders(items []LeaderMenuItem) []LeaderMenuItem {
	for len(items) > 0 && items[len(items)-1].GroupTitle != "" {
		items = items[:len(items)-1]
	}
	return items
}

// LeaderMenuVisibleItems returns the prefix of items that fits in the terminal height.
func LeaderMenuVisibleItems(layout geom.Layout, items []LeaderMenuItem) []LeaderMenuItem {
	maxRows := leaderMenuMaxContentRows(layout)
	if maxRows <= 0 || len(items) == 0 {
		return nil
	}
	for i := 1; i <= len(items); i++ {
		if leaderMenuContentRows(items[:i]) > maxRows {
			return trimLeaderMenuTrailingGroupHeaders(items[:i-1])
		}
	}
	return items
}

// LeaderMenuHiddenActionCount reports action entries omitted because the terminal is too short.
func LeaderMenuHiddenActionCount(layout geom.Layout, items []LeaderMenuItem) int {
	visible := LeaderMenuVisibleItems(layout, items)
	if len(visible) >= len(items) {
		return 0
	}
	return leaderMenuActionCount(items) - leaderMenuActionCount(visible)
}

func leaderMenuActionCount(items []LeaderMenuItem) int {
	n := 0
	for _, it := range items {
		if it.GroupTitle == "" {
			n++
		}
	}
	return n
}

// LeaderMenuRect returns the docked strip above the footer for visible content rows.
func LeaderMenuRect(layout geom.Layout, contentRows int) geom.Rect {
	if contentRows <= 0 || layout.Footer.Height <= 0 {
		return geom.Rect{}
	}
	height := contentRows + 2*leaderMenuMarginRows
	y := layout.Footer.Y - height
	if y < 0 {
		y = 0
		height = layout.Footer.Y
	}
	return geom.Rect{X: 0, Y: y, Width: layout.Width, Height: height}
}

// LeaderMenuMinContentY is the first row the leader menu may occupy (below the menu bar when shown).
func LeaderMenuMinContentY(layout geom.Layout) int {
	if layout.Menu.Height > 0 {
		return layout.Menu.Y + layout.Menu.Height
	}
	return 0
}

// leaderMenuKeys returns the activation rune per item, indexed like items (0 for group
// headers): the pinned Key verbatim, otherwise an auto mnemonic that never collides
// (case-insensitively) with any pinned key, regardless of order. When two items pin the
// exact same rune, only the first keeps it verbatim; the later exact duplicate keeps the
// auto-assigned fallback from dialog.ItemMnemonics (which already skipped the taken letter)
// so both rows stay reachable and display distinct letters. Case variants (f vs F) are not
// duplicates and both stay verbatim.
func leaderMenuKeys(items []LeaderMenuItem) []rune {
	var labels []string
	var configured []rune
	for _, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		labels = append(labels, it.Label)
		configured = append(configured, it.Key)
	}
	actionKeys := dialog.ItemMnemonics(labels, configured)
	seen := map[rune]struct{}{}
	for i, k := range configured {
		if k == 0 {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		actionKeys[i] = k
	}
	keys := make([]rune, len(items))
	actionIdx := 0
	for i, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		keys[i] = actionKeys[actionIdx]
		actionIdx++
	}
	return keys
}

// LeaderMenuIndexForKey returns the action index (skipping group headers) activated by r.
func LeaderMenuIndexForKey(items []LeaderMenuItem, r rune) (int, bool) {
	if r == 0 {
		return 0, false
	}
	keys := leaderMenuKeys(items)
	actionIdx := 0
	for i, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		k := keys[i]
		if it.Key != 0 {
			if k == r {
				return actionIdx, true
			}
		} else if k != 0 && unicode.ToLower(k) == unicode.ToLower(r) {
			return actionIdx, true
		}
		actionIdx++
	}
	return 0, false
}

// leaderMenuShowDirectKeys reports whether direct keybind suffixes may be drawn.
// Suffixes are dropped when the terminal is too short to show every action, or too
// narrow for every visible label and its direct-key hint to fit in a column.
func leaderMenuShowDirectKeys(layout geom.Layout, items []LeaderMenuItem) bool {
	if LeaderMenuHiddenActionCount(layout, items) > 0 {
		return false
	}
	return leaderMenuDirectKeysFitColumns(layout, items)
}

func leaderMenuColumnWidth(layout geom.Layout, grouped bool) int {
	contentWidth := layout.Width - leaderMenuLeftMargin
	cols := leaderMenuItemColumns
	if grouped {
		cols = leaderMenuMacroColumns
	}
	colWidth := contentWidth / cols
	if colWidth < 4 {
		colWidth = 4
	}
	return colWidth
}

func leaderMenuCellPrefixWidth(key rune) int {
	width := 0
	if key != 0 {
		width++
	}
	return width + 3 // space, arrow, space
}

// leaderMenuCellWidth is the width a single item's key+arrow+label(+direct-key
// suffix) needs to render without truncation.
func leaderMenuCellWidth(key rune, label, directKey string) int {
	w := leaderMenuCellPrefixWidth(key) + utf8.RuneCountInString(strings.TrimSpace(label))
	if directKey != "" {
		w += utf8.RuneCountInString(" " + directKey)
	}
	return w
}

func leaderMenuDirectKeyFitsColumn(colWidth int, key rune, label, directKey string) bool {
	if directKey == "" {
		return true
	}
	return leaderMenuCellWidth(key, label, directKey) <= colWidth
}

func leaderMenuDirectKeysFitColumns(layout geom.Layout, items []LeaderMenuItem) bool {
	if layout.Width <= leaderMenuLeftMargin {
		return false
	}
	visible := LeaderMenuVisibleItems(layout, items)
	if len(visible) == 0 {
		return true
	}
	if leaderMenuIsGrouped(visible) {
		rect := LeaderMenuRect(layout, leaderMenuContentRows(visible))
		buckets := leaderMenuSplitByColumn(visible)
		natural := leaderMenuMacroColumnNaturalWidths(buckets, true)
		if _, _, fits := leaderMenuMacroColumnLayout(rect, buckets, true); fits {
			return true
		}
		colWidth := leaderMenuColumnWidth(layout, true)
		for _, w := range natural {
			if w > colWidth {
				return false
			}
		}
		return true
	}
	colWidth := leaderMenuColumnWidth(layout, false)
	keys := leaderMenuKeys(visible)
	for i, it := range visible {
		if it.GroupTitle != "" || it.DirectKey == "" {
			continue
		}
		if !leaderMenuDirectKeyFitsColumn(colWidth, keys[i], it.Label, it.DirectKey) {
			return false
		}
	}
	return true
}

// leaderMenuMacroColumnNaturalWidth is the width a macro column's bucket needs
// to show every item's key+arrow+label(+direct-key suffix) without truncation.
func leaderMenuMacroColumnNaturalWidth(bucket []LeaderMenuItem, showDirectKeys bool) int {
	width := 0
	for _, it := range bucket {
		if it.GroupTitle != "" {
			width = max(width, utf8.RuneCountInString(strings.TrimSpace(it.GroupTitle)))
			continue
		}
		directKey := ""
		if showDirectKeys {
			directKey = it.DirectKey
		}
		width = max(width, leaderMenuCellWidth(it.Key, it.Label, directKey))
	}
	return max(width, 4)
}

// leaderMenuMacroColumnNaturalWidths computes leaderMenuMacroColumnNaturalWidth
// for every bucket.
func leaderMenuMacroColumnNaturalWidths(buckets [leaderMenuMacroColumns][]LeaderMenuItem, showDirectKeys bool) [leaderMenuMacroColumns]int {
	natural := [leaderMenuMacroColumns]int{}
	for i, b := range buckets {
		natural[i] = leaderMenuMacroColumnNaturalWidth(b, showDirectKeys)
	}
	return natural
}

// leaderMenuMacroColumnGap returns the gap to place between every pair of adjacent
// macro columns so columns 1..N-2 sit evenly spaced between column 0 (left-anchored)
// and the last column (right-anchored), and whether that gap is wide enough to use.
func leaderMenuMacroColumnGap(rect geom.Rect, natural [leaderMenuMacroColumns]int) (gap int, ok bool) {
	if rect.Width <= leaderMenuLeftMargin {
		return 0, false
	}
	leftEdge := rect.X + leaderMenuLeftMargin
	rightEdge := rect.X + rect.Width - leaderMenuRightMargin
	sum := 0
	for _, w := range natural {
		sum += w
	}
	numGaps := leaderMenuMacroColumns - 1
	gap = (rightEdge - leftEdge - sum) / numGaps
	return gap, gap >= leaderMenuColumnGap
}

// leaderMenuMacroColumnLayout computes the 4 macro-column x-offsets and widths. With all
// columns in use: column 0 left-anchored, the last column right-anchored (with
// leaderMenuRightMargin blank columns after it), and the columns in between evenly spaced
// across the gap. With fewer non-empty columns: the content width is split into
// leaderMenuMacroColumns equal slots and the used columns sit on consecutive slots,
// centered (2 columns take the two middle slots). Either falls back to uniform division
// when it doesn't fit. fits reports which layout was used.
func leaderMenuMacroColumnLayout(rect geom.Rect, buckets [leaderMenuMacroColumns][]LeaderMenuItem, showDirectKeys bool) (xs [leaderMenuMacroColumns]int, widths [leaderMenuMacroColumns]int, fits bool) {
	natural := leaderMenuMacroColumnNaturalWidths(buckets, showDirectKeys)
	widths = natural
	xBase := rect.X + leaderMenuLeftMargin
	last := leaderMenuMacroColumns - 1

	var used []int
	for i, b := range buckets {
		if len(b) > 0 {
			used = append(used, i)
		}
	}
	if len(used) > 0 && len(used) < leaderMenuMacroColumns {
		return leaderMenuSlotLayout(rect, used, natural)
	}

	gap, fits := leaderMenuMacroColumnGap(rect, natural)
	if fits {
		xs[0] = xBase
		x := xBase + natural[0]
		for i := 1; i < last; i++ {
			x += gap
			xs[i] = x
			x += natural[i]
		}
		xs[last] = rect.X + rect.Width - leaderMenuRightMargin - natural[last]
		return xs, widths, true
	}

	// Fallback: today's uniform division across the full width.
	contentWidth := rect.Width - leaderMenuLeftMargin
	colWidth := max(contentWidth/leaderMenuMacroColumns, 4)
	for i := range xs {
		xs[i] = xBase + i*colWidth
		widths[i] = colWidth
	}
	return xs, widths, false
}

// leaderMenuSlotLayout places the used macro columns (fewer than leaderMenuMacroColumns) on
// consecutive equal-width slots centered in the content width; see leaderMenuMacroColumnLayout.
func leaderMenuSlotLayout(rect geom.Rect, used []int, natural [leaderMenuMacroColumns]int) (xs [leaderMenuMacroColumns]int, widths [leaderMenuMacroColumns]int, fits bool) {
	widths = natural
	leftEdge := rect.X + leaderMenuLeftMargin
	rightEdge := rect.X + rect.Width - leaderMenuRightMargin
	pitch := (rightEdge - leftEdge) / leaderMenuMacroColumns
	start := leftEdge + (rightEdge-leftEdge-len(used)*pitch)/2
	fits = pitch > 0
	for j, col := range used {
		xs[col] = start + j*pitch
		if j < len(used)-1 {
			fits = fits && natural[col]+leaderMenuColumnGap <= pitch
		} else {
			fits = fits && xs[col]+natural[col] <= rightEdge
		}
	}
	if fits {
		return xs, widths, true
	}
	colWidth := max((rect.Width-leaderMenuLeftMargin)/len(used), 4)
	for j, col := range used {
		xs[col] = leftEdge + j*colWidth
		widths[col] = colWidth
	}
	return xs, widths, false
}

// leaderMenuPlacement resolves the drawn strip: its rect and the items that fit in it. ok is false
// when the menu is closed, empty, or the terminal has no room for it.
func leaderMenuPlacement(layout geom.Layout, state LeaderMenuState) (rect geom.Rect, visibleItems []LeaderMenuItem, ok bool) {
	if len(state.Items) == 0 || !state.Open {
		return geom.Rect{}, nil, false
	}
	visibleItems = LeaderMenuVisibleItems(layout, state.Items)
	if len(visibleItems) == 0 {
		return geom.Rect{}, nil, false
	}
	rect = LeaderMenuRect(layout, leaderMenuContentRows(visibleItems))
	minHeight := 2*leaderMenuMarginRows + 1
	if rect.Height < minHeight || rect.Width <= leaderMenuLeftMargin {
		return geom.Rect{}, nil, false
	}
	return rect, visibleItems, true
}

// leaderMenuCell is where one visible item is drawn.
type leaderMenuCell struct {
	X, Y, Width int
	Item        int // index into the visible items
	Action      int // action index (group headers skipped), -1 on a group header
}

// leaderMenuCells lays out items in rect: grouped menus stack each macro column's groups, flat
// menus fill leaderMenuItemColumns columns row by row and skip group headers.
func leaderMenuCells(rect geom.Rect, items []LeaderMenuItem, showDirectKeys bool) []leaderMenuCell {
	cells := make([]leaderMenuCell, 0, len(items))
	yBase := rect.Y + leaderMenuMarginRows
	action := 0
	if leaderMenuIsGrouped(items) {
		xs, widths, _ := leaderMenuMacroColumnLayout(rect, leaderMenuSplitByColumn(items), showDirectKeys)
		colY := [leaderMenuMacroColumns]int{}
		for i := range colY {
			colY[i] = yBase
		}
		for i, it := range items {
			col := leaderMenuClampColumn(it.GroupColumn)
			cell := leaderMenuCell{X: xs[col], Y: colY[col], Width: widths[col], Item: i, Action: -1}
			if it.GroupTitle == "" {
				cell.Action = action
				action++
			}
			cells = append(cells, cell)
			colY[col]++
		}
		return cells
	}
	colWidth := max((rect.Width-leaderMenuLeftMargin)/leaderMenuItemColumns, 4)
	xBase := rect.X + leaderMenuLeftMargin
	y := yBase
	col := 0
	for i, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		cells = append(cells, leaderMenuCell{X: xBase + col*colWidth, Y: y, Width: colWidth, Item: i, Action: action})
		action++
		col++
		if col >= leaderMenuItemColumns {
			col = 0
			y++
		}
	}
	return cells
}

// LeaderMenuActionAt reports whether (x, y) lies on the open leader menu strip and, when it is
// on an action's cell, that action's index (as LeaderMenuIndexForKey returns); action is -1 on
// group headers and empty space.
func LeaderMenuActionAt(layout geom.Layout, state LeaderMenuState, x, y int) (action int, inside bool) {
	rect, visibleItems, ok := leaderMenuPlacement(layout, state)
	if !ok || !rect.Contains(x, y) {
		return -1, false
	}
	for _, cell := range leaderMenuCells(rect, visibleItems, leaderMenuShowDirectKeys(layout, state.Items)) {
		if cell.Action >= 0 && y == cell.Y && x >= cell.X && x < cell.X+cell.Width {
			return cell.Action, true
		}
	}
	return -1, true
}

// DrawLeaderMenu paints the bottom function menu above the footer.
func DrawLeaderMenu(screen tcell.Screen, layout geom.Layout, state LeaderMenuState, styles theme.Theme) {
	rect, visibleItems, ok := leaderMenuPlacement(layout, state)
	if !ok {
		return
	}

	_, surfBG, _ := styles.LeaderMenuSurface.Decompose()
	surface := styles.LeaderMenuSurface
	if surfBG == tcell.ColorDefault {
		surface = styles.LeaderMenuLabel.Background(tcell.ColorDefault)
	}
	primitive.Fill(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, ' ', surface)

	arrow := styles.IconLeaderMenuArrow()
	arrowStyle := styles.LeaderMenuArrow
	if _, ab, _ := arrowStyle.Decompose(); ab == tcell.ColorDefault {
		arrowStyle = arrowStyle.Background(surfBG)
	}
	keyStyle := styles.LeaderMenuKey
	if _, kb, _ := keyStyle.Decompose(); kb == tcell.ColorDefault {
		keyStyle = keyStyle.Background(surfBG)
	}
	labelStyle := styles.LeaderMenuLabel
	if _, lb, _ := labelStyle.Decompose(); lb == tcell.ColorDefault {
		labelStyle = labelStyle.Background(surfBG)
	}
	disabledStyle := styles.LeaderMenuDisabled
	if _, db, _ := disabledStyle.Decompose(); db == tcell.ColorDefault {
		disabledStyle = disabledStyle.Background(surfBG)
	}
	groupStyle := styles.LeaderMenuGroup
	if _, gb, _ := groupStyle.Decompose(); gb == tcell.ColorDefault {
		groupStyle = groupStyle.Background(surfBG)
	}

	showDirectKeys := leaderMenuShowDirectKeys(layout, state.Items)
	keys := leaderMenuKeys(visibleItems)
	for _, cell := range leaderMenuCells(rect, visibleItems, showDirectKeys) {
		it := visibleItems[cell.Item]
		if it.GroupTitle != "" {
			primitive.Text(screen, cell.X, cell.Y, cell.Width, strings.TrimSpace(it.GroupTitle), groupStyle)
			continue
		}
		directKey := it.DirectKey
		if !showDirectKeys {
			directKey = ""
		}
		ks, ls := leaderMenuItemStyles(it, keyStyle, labelStyle, disabledStyle)
		drawLeaderMenuCell(screen, cell.X, cell.Y, cell.Width, keys[cell.Item], arrow, it.Label, directKey, ks, arrowStyle, ls, surface)
	}
}

// leaderMenuItemStyles returns the key and label styles, dimmed for disabled rows.
func leaderMenuItemStyles(it LeaderMenuItem, key, label, disabled tcell.Style) (tcell.Style, tcell.Style) {
	if it.Disabled {
		return disabled, disabled
	}
	return key, label
}

func drawLeaderMenuCell(screen tcell.Screen, x, y, width int, key rune, arrow rune, label, directKey string, keyStyle, arrowStyle, labelStyle, surface tcell.Style) {
	if width <= 0 {
		return
	}
	pos := x
	if key != 0 {
		screen.SetContent(pos, y, key, nil, keyStyle)
		pos++
	}
	if pos >= x+width {
		return
	}
	screen.SetContent(pos, y, ' ', nil, surface)
	pos++
	if pos >= x+width {
		return
	}
	screen.SetContent(pos, y, arrow, nil, arrowStyle)
	pos++
	if pos >= x+width {
		return
	}
	if pos < x+width {
		screen.SetContent(pos, y, ' ', nil, labelStyle)
		pos++
	}
	contentEnd := x + width
	suffix := ""
	suffixWidth := 0
	if directKey != "" {
		suffix = " " + directKey
		suffixWidth = utf8.RuneCountInString(suffix)
	}
	labelWidth := contentEnd - pos - suffixWidth
	if labelWidth < 1 {
		suffix = ""
		labelWidth = contentEnd - pos
	}
	if labelWidth <= 0 {
		return
	}
	text := primitive.TruncateRight(strings.TrimSpace(label), labelWidth)
	primitive.Text(screen, pos, y, labelWidth, text, labelStyle)
	if suffix != "" {
		suffixX := pos + utf8.RuneCountInString(text)
		primitive.Text(screen, suffixX, y, contentEnd-suffixX, suffix, arrowStyle)
	}
}

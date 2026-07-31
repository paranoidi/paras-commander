package ui

import (
	"strings"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
)

const (
	leaderMenuMacroColumns = 3
	leaderMenuItemColumns  = 3
	leaderMenuLeftMargin   = 5
	leaderMenuMarginRows   = 1 // empty surface row above and below content
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

func leaderMenuSplitByColumn(items []LeaderMenuItem) [leaderMenuMacroColumns][]LeaderMenuItem {
	var buckets [leaderMenuMacroColumns][]LeaderMenuItem
	col := 0
	for _, it := range items {
		if it.GroupTitle != "" {
			col = it.GroupColumn
		}
		if col < 0 || col >= leaderMenuMacroColumns {
			col = 0
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

func leaderMenuDisplayKey(items []LeaderMenuItem, i int) rune {
	if i < 0 || i >= len(items) {
		return 0
	}
	if items[i].GroupTitle != "" {
		return 0
	}
	if items[i].Key != 0 {
		return items[i].Key
	}
	var labels []string
	var configured []rune
	for _, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		labels = append(labels, it.Label)
		configured = append(configured, it.Key)
	}
	actionIdx := 0
	for j, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		if j == i {
			shortcuts := dialog.ItemMnemonics(labels, configured)
			if actionIdx < len(shortcuts) {
				return shortcuts[actionIdx]
			}
			return 0
		}
		actionIdx++
	}
	return 0
}

// LeaderMenuIndexForKey returns the action index (skipping group headers) activated by r.
func LeaderMenuIndexForKey(items []LeaderMenuItem, r rune) (int, bool) {
	if r == 0 {
		return 0, false
	}
	actionIdx := 0
	for _, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		if it.Key != 0 && it.Key == r {
			return actionIdx, true
		}
		actionIdx++
	}
	var autoLabels []string
	var autoConfigured []rune
	actionIdx = 0
	var autoIndices []int
	for _, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		if it.Key != 0 {
			actionIdx++
			continue
		}
		autoLabels = append(autoLabels, it.Label)
		autoConfigured = append(autoConfigured, it.Key)
		autoIndices = append(autoIndices, actionIdx)
		actionIdx++
	}
	if len(autoLabels) == 0 {
		return 0, false
	}
	j, ok := dialog.ItemIndexForMnemonic(autoLabels, autoConfigured, r)
	if !ok {
		return 0, false
	}
	return autoIndices[j], true
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

func leaderMenuDirectKeyFitsColumn(colWidth int, key rune, label, directKey string) bool {
	if directKey == "" {
		return true
	}
	prefix := leaderMenuCellPrefixWidth(key)
	suffixWidth := utf8.RuneCountInString(" " + directKey)
	labelWidth := utf8.RuneCountInString(strings.TrimSpace(label))
	return prefix+labelWidth+suffixWidth <= colWidth
}

func leaderMenuDirectKeysFitColumns(layout geom.Layout, items []LeaderMenuItem) bool {
	if layout.Width <= leaderMenuLeftMargin {
		return false
	}
	visible := LeaderMenuVisibleItems(layout, items)
	if len(visible) == 0 {
		return true
	}
	grouped := leaderMenuIsGrouped(visible)
	colWidth := leaderMenuColumnWidth(layout, grouped)
	for i, it := range visible {
		if it.GroupTitle != "" || it.DirectKey == "" {
			continue
		}
		key := it.Key
		if key == 0 {
			key = leaderMenuDisplayKey(visible, i)
		}
		if !leaderMenuDirectKeyFitsColumn(colWidth, key, it.Label, it.DirectKey) {
			return false
		}
	}
	return true
}

// DrawLeaderMenu paints the bottom function menu above the footer.
func DrawLeaderMenu(screen tcell.Screen, layout geom.Layout, state LeaderMenuState, styles theme.Theme) {
	if len(state.Items) == 0 || !state.Open {
		return
	}
	visibleItems := LeaderMenuVisibleItems(layout, state.Items)
	if len(visibleItems) == 0 {
		return
	}
	contentRows := leaderMenuContentRows(visibleItems)
	rect := LeaderMenuRect(layout, contentRows)
	minHeight := 2*leaderMenuMarginRows + 1
	if rect.Height < minHeight || rect.Width <= leaderMenuLeftMargin {
		return
	}

	_, surfBG, _ := styles.LeaderMenuSurface.Decompose()
	surface := styles.LeaderMenuSurface
	if surfBG == tcell.ColorDefault {
		surface = styles.LeaderMenuLabel.Background(tcell.ColorDefault)
	}
	primitive.Fill(screen, primitive.Rect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, ' ', surface)

	arrow := styles.SymbolLeaderMenuArrow()
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
	groupStyle := styles.LeaderMenuGroup
	if _, gb, _ := groupStyle.Decompose(); gb == tcell.ColorDefault {
		groupStyle = groupStyle.Background(surfBG)
	}

	showDirectKeys := leaderMenuShowDirectKeys(layout, state.Items)
	if leaderMenuIsGrouped(visibleItems) {
		drawLeaderMenuGrouped(screen, rect, visibleItems, arrow, showDirectKeys, keyStyle, arrowStyle, labelStyle, groupStyle, surface)
		return
	}
	drawLeaderMenuFlat(screen, rect, visibleItems, arrow, showDirectKeys, keyStyle, arrowStyle, labelStyle, surface)
}

func drawLeaderMenuGrouped(screen tcell.Screen, rect geom.Rect, items []LeaderMenuItem, arrow rune, showDirectKeys bool, keyStyle, arrowStyle, labelStyle, groupStyle, surface tcell.Style) {
	contentWidth := rect.Width - leaderMenuLeftMargin
	macroW := contentWidth / leaderMenuMacroColumns
	if macroW < 4 {
		macroW = 4
	}
	xBase := rect.X + leaderMenuLeftMargin
	yBase := rect.Y + leaderMenuMarginRows
	colY := [leaderMenuMacroColumns]int{}
	for i := range colY {
		colY[i] = yBase
	}
	for i, it := range items {
		col := it.GroupColumn
		if col < 0 || col >= leaderMenuMacroColumns {
			col = 0
		}
		x := xBase + col*macroW
		y := colY[col]
		if it.GroupTitle != "" {
			text := strings.TrimSpace(it.GroupTitle)
			primitive.Text(screen, x, y, macroW, text, groupStyle)
			colY[col] = y + 1
			continue
		}
		key := it.Key
		if key == 0 {
			key = leaderMenuDisplayKey(items, i)
		}
		directKey := it.DirectKey
		if !showDirectKeys {
			directKey = ""
		}
		drawLeaderMenuCell(screen, x, y, macroW, key, arrow, it.Label, directKey, keyStyle, arrowStyle, labelStyle, surface)
		colY[col] = y + 1
	}
}

func drawLeaderMenuFlat(screen tcell.Screen, rect geom.Rect, items []LeaderMenuItem, arrow rune, showDirectKeys bool, keyStyle, arrowStyle, labelStyle, surface tcell.Style) {
	contentWidth := rect.Width - leaderMenuLeftMargin
	colWidth := contentWidth / leaderMenuItemColumns
	if colWidth < 4 {
		colWidth = 4
	}
	xBase := rect.X + leaderMenuLeftMargin
	y := rect.Y + leaderMenuMarginRows
	col := 0
	for i, it := range items {
		if it.GroupTitle != "" {
			continue
		}
		x := xBase + col*colWidth
		directKey := it.DirectKey
		if !showDirectKeys {
			directKey = ""
		}
		drawLeaderMenuCell(screen, x, y, colWidth, leaderMenuDisplayKey(items, i), arrow, it.Label, directKey, keyStyle, arrowStyle, labelStyle, surface)
		col++
		if col >= leaderMenuItemColumns {
			col = 0
			y++
		}
	}
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

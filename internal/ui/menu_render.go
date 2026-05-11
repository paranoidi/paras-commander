package ui

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
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

// drawMenuBarBlank fills the menu row with menu background and no labels (modal overlays block the menu).
func drawMenuBarBlank(screen tcell.Screen, rect Rect, styles theme.Theme, attention, perm string, showMenuBarSpinner bool, spinPhase uint8) {
	primitive.Text(screen, rect.X, rect.Y, rect.Width, "", styles.MenuBar)
	drawMenuBarRightTail(screen, rect, attention, perm, styles.StatusWaitingInput, styles.MenuDetail, styles.PanelSpinner, showMenuBarSpinner, spinPhase)
}

func drawMenuBar(screen tcell.Screen, rect Rect, state menu.State, menus []menu.Definition, styles theme.Theme, attention, perm string, showMenuBarSpinner bool, spinPhase uint8) {
	x := rect.X
	primitive.Text(screen, x, rect.Y, rect.Width, "", styles.MenuBar)
	tailW := menuBarRightTailRuneCount(attention, perm, showMenuBarSpinner)
	clipExclusive := menuBarMenusClipExclusive(rect, tailW)
	for index, menuDefinition := range menus {
		label := " " + menuDefinition.Label + " "
		labelW := utf8.RuneCountInString(label)
		style := styles.MenuBar
		accent := styles.MenuBarAccent
		if state.Open && index == state.ActiveMenu {
			style = styles.MenuBarSelected
		}
		if x >= clipExclusive {
			break
		}
		avail := clipExclusive - x
		if avail <= 0 {
			break
		}
		drawW := min(labelW, avail)
		drawMenuBarLabel(screen, x, rect.Y, drawW, label, menuDefinition.Shortcut, style, accent, state.Open && !state.PulldownOpen)
		if labelW > avail {
			break
		}
		x += labelW + 1
	}
	drawMenuBarRightTail(screen, rect, attention, perm, styles.StatusWaitingInput, styles.MenuDetail, styles.PanelSpinner, showMenuBarSpinner, spinPhase)
}

// menuBarMenusClipExclusive is the first column after the menu label area (before permission tail).
func menuBarMenusClipExclusive(rect Rect, permissionTailRunes int) int {
	if permissionTailRunes <= 0 {
		return rect.X + rect.Width
	}
	permStart := rect.X + rect.Width - menuBarPermRightMargin - permissionTailRunes
	return max(rect.X, permStart-menuBarPermGap)
}

// menuBarMenusEndX returns the column just after menu labels laid out like drawMenuBar (respecting permission clip).
func menuBarMenusEndX(rect Rect, menus []menu.Definition, permissionTailRunes int) int {
	clip := menuBarMenusClipExclusive(rect, permissionTailRunes)
	x := rect.X
	for _, menuDefinition := range menus {
		label := " " + menuDefinition.Label + " "
		labelW := utf8.RuneCountInString(label)
		if x >= clip {
			break
		}
		avail := clip - x
		if avail <= 0 {
			break
		}
		if labelW > avail {
			return x + avail
		}
		x += labelW + 1
	}
	return min(x, clip)
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

	last := rect.X + rect.Width - menuBarPermRightMargin - 1
	if showMenuBarSpinner {
		if last >= rect.X && last < rect.X+rect.Width {
			screen.SetContent(last, rect.Y, MenuBarSpinnerGlyph(spinPhase), nil, spinnerStyle)
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
		for i, r := range attRunes {
			x := startAtt + i
			if x < rect.X {
				continue
			}
			if x >= rect.X+rect.Width {
				break
			}
			screen.SetContent(x, rect.Y, r, nil, alertStyle)
		}
		return
	}
	startAtt := last - len(attRunes) + 1
	for i, r := range attRunes {
		x := startAtt + i
		if x < rect.X {
			continue
		}
		if x >= rect.X+rect.Width {
			break
		}
		screen.SetContent(x, rect.Y, r, nil, alertStyle)
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
			nextStyle = accentGlyphStyle(style, accent)
			highlighted = true
		}
		screen.SetContent(x+column, y, r, nil, nextStyle)
		column++
	}
}

func drawPulldownMenu(screen tcell.Screen, layout Layout, state menu.State, menus []menu.Definition, styles theme.Theme) {
	if !state.PulldownOpen || state.ActiveMenu < 0 || state.ActiveMenu >= len(menus) {
		return
	}
	menuDefinition := menus[state.ActiveMenu]
	if len(menuDefinition.Items) == 0 {
		return
	}

	menuX := menuBarItemX(state.ActiveMenu, menus)
	width := pulldownWidth(menuDefinition)
	if menuX+width > layout.Width {
		menuX = max(0, layout.Width-width)
	}
	rect := Rect{X: menuX, Y: layout.Menu.Y + 1, Width: width, Height: len(menuDefinition.Items) + 2}
	if rect.Y+rect.Height > layout.Footer.Y {
		rect.Height = max(2, layout.Footer.Y-rect.Y)
	}

	primitive.Box(screen, primitive.Rect(rect), styles.MenuDropdownFrame)
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
			nextStyle = accentGlyphStyle(style, accent)
			highlighted = true
		}
		screen.SetContent(x+column, y, r, nil, nextStyle)
		column++
	}
}

func menuBarItemX(activeIndex int, menus []menu.Definition) int {
	x := 0
	for index := 0; index < activeIndex && index < len(menus); index++ {
		x += utf8.RuneCountInString(" "+menus[index].Label+" ") + 1
	}
	return x
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

package app

import (
	"fmt"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/keymap"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

func firstSelectableMenuItem(menuDefinition menu.Definition) int {
	for index, item := range menuDefinition.Items {
		if !item.Separator {
			return index
		}
	}
	return 0
}

func wrap(value, count int) int {
	if count <= 0 {
		return 0
	}
	value %= count
	if value < 0 {
		value += count
	}
	return value
}

func (a *App) handleQuickFilterFunctionKey(event *tcell.EventKey) bool {
	viewportRows := a.activeViewportRows()
	a.activePanel().CancelFilter(viewportRows)
	if event.Key() == tcell.KeyF9 {
		a.openMenu()
		return false
	}
	label, _ := menu.FunctionKeyLabelByKey(event.Key())
	if def, item, ok := menu.FindItemByFKeyLabel(menu.ActiveDefinitions(a.model.MenuDefinitions), label); ok {
		return a.activateMenuSelection(def, item)
	}
	if id, ok := a.keys.Global.Lookup(event); ok && id == keymap.ActionAppUserMenu {
		a.openUserMenu()
		return false
	}
	return false
}

func (a *App) handleMenuKey(event *tcell.EventKey) bool {
	switch event.Key() {
	case tcell.KeyEsc, tcell.KeyCtrlLeftSq:
		if a.model.Menu.PulldownOpen {
			a.model.Menu.PulldownOpen = false
		} else {
			a.closeMenu()
		}
	case tcell.KeyF9:
		a.closeMenu()
	case tcell.KeyLeft:
		a.moveMenu(-1)
	case tcell.KeyRight:
		a.moveMenu(1)
	case tcell.KeyUp:
		if a.model.Menu.PulldownOpen {
			// Up from first selectable item closes pulldown, stays on same menu.
			menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
			if a.model.Menu.ActiveMenu >= 0 && a.model.Menu.ActiveMenu < len(menus) {
				firstIdx := firstSelectableMenuItem(menus[a.model.Menu.ActiveMenu])
				if a.model.Menu.SelectedItem == firstIdx {
					a.model.Menu.PulldownOpen = false
					break
				}
			}
			a.moveMenuItem(-1)
		}
	case tcell.KeyDown:
		if a.model.Menu.PulldownOpen {
			a.moveMenuItem(1)
		} else {
			a.model.Menu.PulldownOpen = true
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menu.ActiveDefinitions(a.model.MenuDefinitions)[a.model.Menu.ActiveMenu])
		}
	case tcell.KeyEnter:
		if a.model.Menu.PulldownOpen {
			return a.activateMenuItem()
		}
		a.model.Menu.PulldownOpen = true
		a.model.Menu.SelectedItem = firstSelectableMenuItem(menu.ActiveDefinitions(a.model.MenuDefinitions)[a.model.Menu.ActiveMenu])
	case tcell.KeyRune:
		if event.Modifiers() == tcell.ModNone && event.Rune() == '\x1b' {
			if a.model.Menu.PulldownOpen {
				a.model.Menu.PulldownOpen = false
			} else {
				a.closeMenu()
			}
			break
		}
		if keymap.AltLetterModifiers(event.Modifiers()) {
			if a.selectTopMenuShortcut(event.Rune()) {
				a.model.Menu.PulldownOpen = true
				break
			}
		}
		if a.model.Menu.PulldownOpen {
			// Pulldown open: plain letters activate pulldown item shortcuts only.
			if a.selectMenuShortcut(event.Rune()) {
				return a.activateMenuItem()
			}
		} else {
			// Menu bar active (no pulldown): plain letters open the matching
			// top menu's pulldown. Pulldown item shortcuts are not active here.
			if a.openMenuByShortcut(event.Rune()) {
				a.model.Menu.PulldownOpen = true
			}
		}
	}
	return false
}

func (a *App) openMenu() {
	if a.model.ViewMode == ui.ViewFilePreview {
		return
	}
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		a.model.Menu.ActiveMenu = menu.DefaultIndex()
	}
	a.model.Menu.Open = true
	a.model.Menu.PulldownOpen = false
	a.model.Menu.SelectedItem = 0
}

func (a *App) openMenuByShortcut(shortcut rune) bool {
	if a.model.ViewMode == ui.ViewFilePreview {
		return false
	}
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	for index, def := range menus {
		if def.Shortcut != 0 && unicode.ToLower(def.Shortcut) == unicode.ToLower(shortcut) {
			a.model.Menu.ActiveMenu = index
			a.model.Menu.Open = true
			a.model.Menu.PulldownOpen = true
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[index])
			return true
		}
	}
	return false
}

func (a *App) closeMenu() {
	a.model.Menu.Open = false
	a.model.Menu.PulldownOpen = false
	a.model.Menu.SelectedItem = 0
}

func (a *App) moveMenu(delta int) {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if len(menus) == 0 {
		return
	}
	a.model.Menu.ActiveMenu = wrap(a.model.Menu.ActiveMenu+delta, len(menus))
	if a.model.Menu.PulldownOpen {
		a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[a.model.Menu.ActiveMenu])
	} else {
		a.model.Menu.SelectedItem = 0
	}
}

func (a *App) moveMenuItem(delta int) {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		return
	}
	count := len(menus[a.model.Menu.ActiveMenu].Items)
	if count == 0 {
		a.model.Menu.SelectedItem = 0
		return
	}
	next := a.model.Menu.SelectedItem
	for range count {
		next = wrap(next+delta, count)
		if !menus[a.model.Menu.ActiveMenu].Items[next].Separator {
			a.model.Menu.SelectedItem = next
			return
		}
	}
	a.model.Menu.SelectedItem = 0
}

func (a *App) activateMenuItem() bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		a.closeMenu()
		return false
	}
	mbar := menus[a.model.Menu.ActiveMenu]
	items := mbar.Items
	if a.model.Menu.SelectedItem < 0 || a.model.Menu.SelectedItem >= len(items) || items[a.model.Menu.SelectedItem].Separator {
		a.closeMenu()
		return false
	}
	quit := a.activateMenuSelection(mbar, items[a.model.Menu.SelectedItem])
	a.closeMenu()
	return quit
}

func (a *App) activateMenuSelection(def menu.Definition, item menu.Item) bool {
	if def.PanelScope != menu.PanelScopeNone {
		a.activateScopedPanelMenu(def.PanelScope, item)
		return false
	}
	switch def.ID {
	case menu.TopCommand, menu.TopDisplay, menu.TopDev:
		a.dispatch(item.Action)
	case menu.TopFile:
		switch item.Action {
		case keymap.ActionAppQuit:
			return a.handleQuit()
		case keymap.ActionAppQuitImmediate:
			return a.handleQuitImmediate()
		case keymap.ActionPanelSelectGroup:
			a.openGroupSelect("select", "panel")
		case keymap.ActionPanelUnselectGroup:
			a.openGroupSelect("unselect", "panel")
		case keymap.ActionPanelInvertSelection:
			a.activePanel().InvertSelection()
			a.setTransientMessage("Selection inverted", ui.MessageUrgencyInfo)
		case keymap.ActionCopy:
			a.dialogCtrl.ActivateCopyAction()
			return false
		case keymap.ActionMove:
			a.dialogCtrl.ActivateMoveAction()
			return false
		case keymap.ActionFileFlatten:
			a.dialogCtrl.OpenFlattenDialog()
			return false
		default:
			a.dispatchFileMenuItem(item)
		}
	case menu.TopOptions:
		switch item.Action {
		case keymap.ActionUIOpenTheme:
			a.openThemeDialog()
		case keymap.ActionUIOpenConfig:
			a.openConfigDialog()
		case keymap.ActionUICalibrateDebounce:
			a.openDebounceCalibrateDialog()
		default:
			a.setUnsupportedMessage(item.Label)
		}
	case menu.TopJobs, menu.TopCommands, menu.TopMessages, menu.TopDedup:
		a.dispatch(item.Action)
	default:
		a.setUnsupportedMessage(item.Label)
	}
	return false
}

func (a *App) activateScopedPanelMenu(panelScope int, item menu.Item) {
	target := a.panelByID(panelScope)
	label := panelLabel(panelScope)
	switch item.Action {
	case keymap.ActionPanelSortDialog:
		a.openSortDialogForPanel(panelScope)
	case keymap.ActionPanelToggleHidden:
		if err := a.toggleHiddenGlobal(); err != nil {
			a.setErrorMessage(label+" toggle hidden failed", err)
			return
		}
	case keymap.ActionPanelRefresh:
		if err := target.Refresh(a.panelViewportRows(panelScope)); err != nil {
			a.setErrorMessage(label+" refresh failed", err)
			return
		}
		if panelScope == ui.PrimaryPanel {
			a.requestVolumeSpaceRefreshAsync(ui.SecondaryPanel)
		} else {
			a.requestVolumeSpaceRefreshAsync(ui.PrimaryPanel)
		}
		a.dialogCtrl.SyncOpenPathInputsAfterFSChange()
		a.setTransientMessage(label+" refreshed", ui.MessageUrgencyInfo)
	case keymap.ActionPanelDiskUsageScan:
		a.startDiskUsageScanForPanel(panelScope)
	case keymap.ActionPanelHistoryDialog:
		a.openHistoryDialog(panelScope)
	case keymap.ActionPanelFindDialog:
		a.findCtrl.OpenDialog(panelScope)
	case keymap.ActionPanelComparePanels:
		a.openComparePanels()
	case keymap.ActionPanelFindDuplicates:
		a.openFindDuplicates()
	case keymap.ActionPanelExternalBrowser:
		a.openPanelPathInExternalBrowser(panelScope)
	case keymap.ActionPanelListingFormatDialog:
		p := a.panelByID(panelScope)
		if p.CarouselMode {
			a.setTransientMessage("Listing format is not available in carousel view", ui.MessageUrgencyInfo)
			break
		}
		a.openListingFormatDialogForPanel(panelScope)
	case keymap.ActionPanelToggleCarousel:
		p := a.panelByID(panelScope)
		p.CarouselMode = !p.CarouselMode
		if p.CarouselMode {
			p.SetListLayout(panel.ListLayoutFlat, a.panelViewportRows(panelScope))
			a.model.ActivePanel = panelScope
		} else {
			a.previewCtrl.ClearCarouselPreviewNavCoalesce()
			a.previewCtrl.CloseCarouselFilePreview()
		}
		onOff := "off"
		if p.CarouselMode {
			onOff = "on"
		}
		label := panelLabel(panelScope)
		a.setTransientMessage(fmt.Sprintf("%s carousel view: %s", label, onOff), ui.MessageUrgencyInfo)
	case keymap.ActionPanelToggleTree:
		p := a.panelByID(panelScope)
		if a.toggleTreeForPanel(p, a.panelViewportRows(panelScope)) {
			a.setTransientMessage(panelLabel(panelScope)+" tree view is not available in carousel view", ui.MessageUrgencyInfo)
		}
	case keymap.ActionPanelMeta:
		a.metaCtrl.OpenDialog(panelScope)
	case keymap.ActionFileQuickView:
		a.previewCtrl.HandleQuickViewToggle()
	case keymap.ActionRemoteSFTPLink:
		a.openSFTPConnectDialogForPanel(panelScope)
	default:
		a.setUnsupportedMessage(item.Label)
	}
}

func (a *App) selectTopMenuShortcut(shortcut rune) bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	for index, def := range menus {
		if def.Shortcut != 0 && unicode.ToLower(def.Shortcut) == unicode.ToLower(shortcut) && index != a.model.Menu.ActiveMenu {
			a.model.Menu.ActiveMenu = index
			a.model.Menu.SelectedItem = firstSelectableMenuItem(menus[index])
			return true
		}
	}
	return false
}

func (a *App) selectMenuShortcut(shortcut rune) bool {
	menus := menu.ActiveDefinitions(a.model.MenuDefinitions)
	if a.model.Menu.ActiveMenu < 0 || a.model.Menu.ActiveMenu >= len(menus) {
		return false
	}
	var caseInsensitive = -1
	for index, item := range menus[a.model.Menu.ActiveMenu].Items {
		if item.Separator || item.Shortcut == 0 {
			continue
		}
		if item.Shortcut == shortcut {
			a.model.Menu.SelectedItem = index
			return true
		}
		if caseInsensitive < 0 && unicode.ToLower(item.Shortcut) == unicode.ToLower(shortcut) {
			caseInsensitive = index
		}
	}
	if caseInsensitive >= 0 {
		a.model.Menu.SelectedItem = caseInsensitive
		return true
	}
	return false
}

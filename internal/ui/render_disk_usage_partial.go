package ui

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
)

// PaintDiskUsageBrowserPanelsOnly repaints file-list panels in disk-usage scan scope and
// the menu-bar activity spinner without a full terminal repaint. layout must match the
// current terminal size (same rules as Render). Returns false when the caller should
// fall back to ui.Render (non-browser view or terminal too small).
func PaintDiskUsageBrowserPanelsOnly(screen tcell.Screen, layout Layout, model Model, styles theme.Theme) bool {
	if model.ViewMode != ViewBrowser {
		return false
	}
	if layout.TooSmall {
		return false
	}

	painted := paintBrowserPanelsInScope(screen, layout, model, styles, model.showPanelDiskUsage)
	if DrawMenuBarSpinnerOnly(screen, layout, model, styles) {
		painted = true
	}
	return painted
}

// paintBrowserPanelsInScope redraws twin-panel file lists (and visible selections strips)
// for panelIDs where inScope returns true. Skips inactive-column file preview surfaces.
func paintBrowserPanelsInScope(
	screen tcell.Screen,
	layout Layout,
	model Model,
	styles theme.Theme,
	inScope func(panelID int) bool,
) bool {
	if !inScope(PrimaryPanel) && !inScope(SecondaryPanel) {
		return false
	}

	previewTheme := model.ThemeDialog.Open
	primaryChromeBlocked := model.PanelsChromeBlocked() && !previewTheme
	chromeBlocked := model.PanelsChromeBlocked()
	primaryFileListFocus := previewTheme || (model.ActivePanel == PrimaryPanel && model.renderSubFocus() == SubFocusFileList)
	secondaryFileListFocus := model.ActivePanel == SecondaryPanel && model.renderSubFocus() == SubFocusFileList

	leftStripCount := model.Primary.SelectionsStripCount()
	rightStripCount := model.Secondary.SelectionsStripCount()
	leftStripN := SelectionsStripLayoutItemCountFromCount(leftStripCount, PrimaryPanel, model.ActivePanel, previewTheme)
	rightStripN := SelectionsStripLayoutItemCountFromCount(rightStripCount, SecondaryPanel, model.ActivePanel, previewTheme)
	primaryFile := FileListFrameWithStripCount(layout.Primary, leftStripN, model.SelectionsPanelMaxRows)
	secondaryFile := FileListFrameWithStripCount(layout.Secondary, rightStripN, model.SelectionsPanelMaxRows)
	_, leftStrip := SplitPanelColumn(layout.Primary, leftStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)
	_, rightStrip := SplitPanelColumn(layout.Secondary, rightStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)

	primarySelectionsBottomHint := leftStripCount > 0 && leftStripN == 0
	secondarySelectionsBottomHint := rightStripCount > 0 && rightStripN == 0
	leftStripVisible := leftStrip.Height > 0
	rightStripVisible := rightStrip.Height > 0
	primarySelectionSizeOnFileBottom := model.Primary.SelectedPathCount() > 0 && !leftStripVisible
	secondarySelectionSizeOnFileBottom := model.Secondary.SelectedPathCount() > 0 && !rightStripVisible
	leftSelectionSizeOnStripBottom := model.Primary.SelectedPathCount() > 0 && leftStripVisible
	rightSelectionSizeOnStripBottom := model.Secondary.SelectedPathCount() > 0 && rightStripVisible

	inactiveID := model.inactivePanelID()
	showLeftPreview := layout.Primary.Width > 0 && inactiveID == PrimaryPanel && model.InactiveColumnShowsFilePreview(PrimaryPanel)
	showRightPreview := layout.Secondary.Width > 0 && inactiveID == SecondaryPanel && model.InactiveColumnShowsFilePreview(SecondaryPanel)

	primaryOtherPanelPath := model.Secondary.PathString()
	secondaryOtherPanelPath := model.Primary.PathString()
	syncDriver := model.SyncDriverPanelID()
	quickViewDriver := model.QuickViewDriverPanelID()

	painted := false
	var cursorNameHintFallback CursorNameHintFallback
	if inScope(PrimaryPanel) && layout.Primary.Width > 0 && !showLeftPreview {
		drawPanel(screen, primaryFile, model.PanelForFileListRender(PrimaryPanel),
			PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
			PanelContext{
				PanelID: PrimaryPanel, FileListActive: primaryFileListFocus,
				CursorRowActive: primaryFileListFocus && !model.QuickAction.Open, ChromeBlocked: primaryChromeBlocked,
				ActivePanel: model.ActivePanel, OtherPanelPath: primaryOtherPanelPath,
				HideInactivePanel: model.HideInactivePanel, SyncDriverPanelID: syncDriver, QuickViewDriverPanelID: quickViewDriver,
				SplitOrientation: model.SplitOrientation, SelectionsBottomHint: primarySelectionsBottomHint,
				ShowSelectionSizeOnBottom: primarySelectionSizeOnFileBottom,
				CursorNameHintFallbackOut: cursorNameHintFallbackOut(primaryFileListFocus, &cursorNameHintFallback),
				CursorNameHintPinnedOut:   model.CursorNameHintPinOutPrimary,
			},
			PanelDisplayConfig{
				ShowIcons: model.ShowFileIcons, UserHomeDir: model.UserHomeDir,
				Painter: model.DiskUsage, DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints,
				DiskUsageGoduIgnore: model.DiskUsageGoduIgnore, ShowDiskUsage: model.showPanelDiskUsage(PrimaryPanel),
				JobMarks: model.JobPathMarks, MetaColumns: model.MetaResults[PrimaryPanel],
				ShrunkenShowsNameOnly: model.ShrunkenShowsNameOnly, ScrollbarShowInactive: model.PanelScrollbarInactive,
				CarouselLayout: model.CarouselLayout, CarouselFilePreview: model.CarouselFilePreviewDraw,
			})
		painted = true
		if leftStrip.Height > 0 {
			leftStripFocused := model.ActivePanel == PrimaryPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, leftStrip, model.Primary, leftStripFocused, primaryChromeBlocked, SelectionsStripOpts{
				Styles: styles, UserHomeDir: model.UserHomeDir, Painter: model.DiskUsage,
				DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints, DiskUsageGoduIgnore: model.DiskUsageGoduIgnore,
				ShowSelectionSizeOnBottom: leftSelectionSizeOnStripBottom, ScrollbarStyle: model.PanelScrollbar,
				ScrollbarShowInactive: model.PanelScrollbarInactive, PanelFileListActive: primaryFileListFocus,
			})
		}
	}
	if inScope(SecondaryPanel) && layout.Secondary.Width > 0 && !showRightPreview {
		drawPanel(screen, secondaryFile, model.PanelForFileListRender(SecondaryPanel),
			PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
			PanelContext{
				PanelID: SecondaryPanel, FileListActive: secondaryFileListFocus,
				CursorRowActive: secondaryFileListFocus && !model.QuickAction.Open, ChromeBlocked: chromeBlocked,
				ActivePanel: model.ActivePanel, OtherPanelPath: secondaryOtherPanelPath,
				HideInactivePanel: model.HideInactivePanel, SyncDriverPanelID: syncDriver, QuickViewDriverPanelID: quickViewDriver,
				SplitOrientation: model.SplitOrientation, SelectionsBottomHint: secondarySelectionsBottomHint,
				ShowSelectionSizeOnBottom: secondarySelectionSizeOnFileBottom,
				CursorNameHintFallbackOut: cursorNameHintFallbackOut(secondaryFileListFocus, &cursorNameHintFallback),
				CursorNameHintPinnedOut:   model.CursorNameHintPinOutSecondary,
			},
			PanelDisplayConfig{
				ShowIcons: model.ShowFileIcons, UserHomeDir: model.UserHomeDir,
				Painter: model.DiskUsage, DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints,
				DiskUsageGoduIgnore: model.DiskUsageGoduIgnore, ShowDiskUsage: model.showPanelDiskUsage(SecondaryPanel),
				JobMarks: model.JobPathMarks, MetaColumns: model.MetaResults[SecondaryPanel],
				ShrunkenShowsNameOnly: model.ShrunkenShowsNameOnly, ScrollbarShowInactive: model.PanelScrollbarInactive,
				CarouselLayout: model.CarouselLayout, CarouselFilePreview: model.CarouselFilePreviewDraw,
			})
		painted = true
		if rightStrip.Height > 0 {
			rightStripFocused := model.ActivePanel == SecondaryPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, rightStrip, model.Secondary, rightStripFocused, chromeBlocked, SelectionsStripOpts{
				Styles: styles, UserHomeDir: model.UserHomeDir, Painter: model.DiskUsage,
				DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints, DiskUsageGoduIgnore: model.DiskUsageGoduIgnore,
				ShowSelectionSizeOnBottom: rightSelectionSizeOnStripBottom, ScrollbarStyle: model.PanelScrollbar,
				ScrollbarShowInactive: model.PanelScrollbarInactive, PanelFileListActive: secondaryFileListFocus,
			})
		}
	}
	if painted {
		drawCursorNameHintScreenFallback(screen, layout, &cursorNameHintFallback, model.TerminalPanel.Visible)
	}
	return painted
}

// PaintBrowserListNavPanelOnly repaints one browser file-list column (and its selections strip
// when visible) without touching the other twin panel or the rest of the frame.
func PaintBrowserListNavPanelOnly(screen tcell.Screen, layout Layout, model Model, styles theme.Theme, panelID int) bool {
	if model.ViewMode != ViewBrowser || layout.TooSmall {
		return false
	}
	if panelID != PrimaryPanel && panelID != SecondaryPanel {
		return false
	}
	return paintBrowserPanelsInScope(screen, layout, model, styles, func(id int) bool { return id == panelID })
}

package ui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

const (
	LeftPanel = iota
	RightPanel
)

// SubFocus areas within the active browser column (file list vs selections strip),
// or keyboard focus on the inactive-column file preview when it is open.
const (
	SubFocusFileList = iota
	SubFocusSelectionsStrip
	// SubFocusInactivePreview is only used while FilePreview is open (inactive column shows preview).
	SubFocusInactivePreview
)

// Model is the renderable subset of application state.
type Model struct {
	Left           panel.State
	Right          panel.State
	ActivePanel    int
	ActiveSubFocus int // SubFocus*; applies to ActivePanel when ViewBrowser.
	// HideInactivePanel gives the active column full width and hides the inactive twin panel.
	HideInactivePanel bool
	// SyncFollowEnabled gates latched panel sync. When true, SyncFollowPanel
	// (LeftPanel or RightPanel) names the driver whose caret moves auto-load
	// the highlighted directory into the inactive panel.
	SyncFollowEnabled bool
	SyncFollowPanel   int
	// SelectionsPanelMaxRows caps visible rows in the selections strip (0 = use app default).
	SelectionsPanelMaxRows int
	ViewMode               ViewMode
	JobsView               JobsViewState
	JobsList               []JobEntry
	// JobPathMarks is the browser file-list job glyph snapshot (progress fields omitted).
	JobPathMarks []JobPathMark
	JobActivity  map[string][]string
	CommandsView CommandsViewState
	CommandsList []CommandRunEntry
	// CommandsDisplay is a mutex-backed snapshot refreshed in App.render for ViewCommands (avoids races with worker updates).
	CommandsDisplay []CommandRunEntry
	MessagesView    MessagesViewState
	MessageLog      []MessageLogEntry
	// HideMenuBar mirrors !ui.show_menu_bar: when true, the top menu row is omitted and panels extend upward.
	HideMenuBar bool
	// ShowFileIcons mirrors ui.show_file_icons (Nerd Font glyphs before file names).
	ShowFileIcons bool
	// PanelZoomEnabled mirrors effective zoom for layout (saved [ui].zoom_active_panel plus optional
	// runtime-only override in App.render), suppressed while quick view / file preview uses the split,
	// and suppressed on wide terminals when [ui].zoom_active_panel_disabled_above_width > 0.
	// Carousel view on the active panel always enables zoom (ignores preference, override, and width gate).
	// Layout uses it only in the file browser; see PanelZoomSplitsColumns.
	PanelZoomEnabled bool
	// PanelZoomActivePercent / PanelZoomInactivePercent mirror [ui] panel_zoom_* (sum 100 when zoom enabled).
	PanelZoomActivePercent   int
	PanelZoomInactivePercent int
	// ShrunkenShowsNameOnly mirrors ui.shrunken_shows_name_only (narrow panels may hide trailing listing columns).
	ShrunkenShowsNameOnly bool
	// JobsThroughputChartEnabled mirrors [jobs].throughput_chart_enabled (strip + graph off when false).
	JobsThroughputChartEnabled bool
	// UserHomeDir is filepath.Clean(os.UserHomeDir()); empty skips ~ substitution in panel titles.
	UserHomeDir string
	// DiskUsageShown enables proportional disk-usage bars after the user starts a scan.
	DiskUsageShown bool
	// DiskUsagePanelID stores the panel (LeftPanel/RightPanel) that initiated the current disk usage scan (pending tint only).
	DiskUsagePanelID int
	// DiskUsageScanOrigin / DiskUsageScanRoots define the active scan scope (origin listing + queued child roots).
	// Bars and idle disk-total sort apply on either panel while its cwd is in this scope.
	DiskUsageScanOrigin string
	DiskUsageScanRoots  []string
	// DiskUsage provides cached sizes (nil disables painting even when DiskUsageShown is true).
	DiskUsage DiskUsagePainter
	// DiskUsageDescendIntoMountPoints mirrors config disk_usage_descend_into_mount_points (cross-mount subtree scans).
	DiskUsageDescendIntoMountPoints bool
	// DiskUsageGoduIgnore is optional basename ignore (~/.goduignore); nil if unavailable.
	DiskUsageGoduIgnore func(string) bool
	// MenuBarActivitySpinner requests the busy spinner glyph at the menu-bar trailing edge (set by App.render).
	MenuBarActivitySpinner bool
	// SpinPhase advances while the menu-bar activity spinner animates (braille glyph sequence).
	SpinPhase           uint8
	Menu                menu.State
	MenuDefinitions     []menu.Definition
	ThemeDialog         ThemeDialogState
	ConfigDialog        ConfigDialogState
	SortDialog          SortDialogState
	ListingFormatDialog ListingFormatDialogState
	GroupSelect         GroupSelectState
	PathPicker          PathPickerState
	HistoryDialog       HistoryDialogState
	SFTPConnectDialog   SFTPConnectDialogState
	FindDialog          FindDialogState
	MetaDialog          MetaDialogState
	UserMenu            UserMenuDialogState
	// MetaResults holds per-panel command output keyed by entry path (nil = meta not active).
	MetaResults [2]map[string]string
	// FilePreview is the live inactive-panel preview state (mutate only under App.commandsMu).
	FilePreview FilePreviewState
	// FilePreviewDraw is a snapshot copied in App.render before ui.Render (no locks in ui).
	FilePreviewDraw FilePreviewState
	// QuickViewEnabled is true while Shift+F3 / menu "Quick view" is latched for QuickViewPanel.
	QuickViewEnabled bool
	// QuickViewPanel is the driver panel (LeftPanel or RightPanel) when QuickViewEnabled, else -1.
	QuickViewPanel int
	// QuickViewDirOverlay holds a transient directory listing for quick-view directory preview (paint only).
	QuickViewDirOverlay panel.State
	// QuickViewDirOverlayActive is true when QuickViewDirOverlay should replace the inactive file list.
	QuickViewDirOverlayActive bool
	// QuickViewDirOverlayPanelID is LeftPanel or RightPanel for the inactive column, or -1 when inactive.
	QuickViewDirOverlayPanelID int
	// FullscreenFilePreview is the full-screen file view state (mutate only under App.commandsMu).
	FullscreenFilePreview FilePreviewState
	// FullscreenFilePreviewDraw is a snapshot for ViewFilePreview rendering.
	FullscreenFilePreviewDraw FilePreviewState
	HelpView                  HelpViewState
	FileDialog                FileDialogState
	TransferDialog            TransferDialogState
	FlattenDialog             FlattenDialogState
	ConflictDialog            ConflictDialogState
	HostKeyDialog             HostKeyDialogState
	QuitConfirm               QuitConfirmState
	StashRestoreDialog        StashRestoreDialogState
	MessageDialog             MessageDialogState
	Message                   string
	MessageUrgency            MessageUrgency
	FooterKeys                []menu.FunctionKey
	// MenuBarPermission is Unix mode text for the active panel cursor row (e.g. "drwxr-xr-x"); empty when none.
	MenuBarPermission string
	// MenuBarJobsAttention is the core jobs/conflict label (e.g. "! 1"); the menu bar pads it with
	// spaces on both sides for themed backgrounds and separates it from the activity spinner.
	MenuBarJobsAttention string
	// MenuBarJobs is the jobs queue + progress snapshot for the menu-bar gap (App.render).
	MenuBarJobs MenuBarJobsStrip
}

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer (see package dialog).

// PrimaryModal returns the active primary modal, in the same priority order as Render.
func (m Model) PrimaryModal() PrimaryModal {
	switch {
	case m.ThemeDialog.Open:
		return PrimaryModalTheme
	case m.ConflictDialog.Open:
		return PrimaryModalConflict
	case m.TransferDialog.Open:
		return PrimaryModalTransfer
	case m.FlattenDialog.Open:
		return PrimaryModalFlatten
	case m.QuitConfirm.Open:
		return PrimaryModalQuit
	default:
		return PrimaryModalNone
	}
}

// SyncDriverPanelID returns the LeftPanel/RightPanel id that drives latched panel sync,
// or -1 when sync is disabled. The result is intended for renderers that need a sentinel
// they can compare against the panel they are about to draw.
func (m Model) SyncDriverPanelID() int {
	if !m.SyncFollowEnabled {
		return -1
	}
	if m.SyncFollowPanel != LeftPanel && m.SyncFollowPanel != RightPanel {
		return -1
	}
	return m.SyncFollowPanel
}

// showPanelDiskUsage is true when proportional disk-usage bars apply to panelID's listing.
func (m Model) showPanelDiskUsage(panelID int) bool {
	if !m.DiskUsageShown || m.DiskUsage == nil {
		return false
	}
	listingPath := ""
	switch panelID {
	case LeftPanel:
		listingPath = m.Left.PathString()
	case RightPanel:
		listingPath = m.Right.PathString()
	default:
		return false
	}
	return panel.ListingPathInDiskUsageScanScope(listingPath, m.DiskUsageScanOrigin, m.DiskUsageScanRoots)
}

// quickViewDriverPanel resolves the latched quick-view driver panel.
func (m Model) quickViewDriverPanel() int {
	if !m.QuickViewEnabled {
		return -1
	}
	if m.QuickViewPanel == LeftPanel || m.QuickViewPanel == RightPanel {
		return m.QuickViewPanel
	}
	if m.ActivePanel == LeftPanel || m.ActivePanel == RightPanel {
		return m.ActivePanel
	}
	return -1
}

// QuickViewDisplayActive is true when the inactive column should show quick-view preview.
func (m Model) QuickViewDisplayActive() bool {
	if !m.QuickViewEnabled {
		return false
	}
	driver := m.quickViewDriverPanel()
	return driver >= 0 && m.ActivePanel == driver
}

// QuickViewDriverPanelID returns the LeftPanel/RightPanel id that shows the quick-view
// bottom indicator while quick view is latched, or -1 when disabled.
func (m Model) QuickViewDriverPanelID() int {
	return m.quickViewDriverPanel()
}

// PanelForFileListRender returns the panel state to paint in the file list. During quick-view
// directory preview the inactive column uses QuickViewDirOverlay; real Left/Right paths stay
// unchanged for cross-panel open indicators and for restore when quick view is turned off.
func (m Model) PanelForFileListRender(panelID int) panel.State {
	if m.QuickViewDisplayActive() && m.QuickViewDirOverlayActive && panelID == m.QuickViewDirOverlayPanelID {
		return m.QuickViewDirOverlay
	}
	switch panelID {
	case LeftPanel:
		return m.Left
	case RightPanel:
		return m.Right
	default:
		return panel.State{}
	}
}

// PanelsChromeBlocked reports when file/jobs panel chrome should use panel.blocked.*
// styles because a menu or modal has taken focus.
func (m Model) PanelsChromeBlocked() bool {
	if m.Menu.Open {
		return true
	}
	if m.ModalDialogOpen() {
		return true
	}
	return false
}

// ModalDialogOpen reports modals that block normal navigation and hide the menu bar row.
func (m Model) ModalDialogOpen() bool {
	if m.PrimaryModal() != PrimaryModalNone {
		return true
	}
	if m.SortDialog.Open || m.ListingFormatDialog.Open || m.ConfigDialog.Open || m.GroupSelect.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.SFTPConnectDialog.Open || m.FindDialog.Open || m.MetaDialog.Open || m.HelpView.Open || m.FileDialog.Open || m.HostKeyDialog.Open || m.MessageDialog.Open || m.StashRestoreDialog.Open || m.UserMenu.Open {
		return true
	}
	return false
}

// QuickFilterStartBlocked reports UI states where the quick filter must not open from a plain printable key
// in normal input mode (same modal/menu set as the legacy shouldStartFilter guard).
func (m Model) QuickFilterStartBlocked() bool {
	if m.Menu.Open {
		return true
	}
	return m.MessageDialog.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.SFTPConnectDialog.Open || m.FindDialog.Open ||
		m.MetaDialog.Open || m.ThemeDialog.Open || m.SortDialog.Open ||
		m.ListingFormatDialog.Open ||
		m.ConfigDialog.Open || m.GroupSelect.Open || m.FileDialog.Open || m.HostKeyDialog.Open ||
		m.TransferDialog.Open || m.FlattenDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.StashRestoreDialog.Open || m.UserMenu.Open
}

// AuxiliaryViewDialogKeysBlocked reports transfer/conflict/quit dialogs plus the pulldown menu that block
// dedicated Jobs/Commands view keyboard handling. inputMode checks this only after earlier cases have ruled
// out other modals.
func (m Model) AuxiliaryViewDialogKeysBlocked() bool {
	return m.TransferDialog.Open || m.FlattenDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.StashRestoreDialog.Open || m.Menu.Open
}

// MenuBarLayoutReserved is true when the top row is reserved for the menu strip (config show_menu_bar).
func (m Model) MenuBarLayoutReserved() bool {
	return !m.HideMenuBar
}

// MenuBarInteractive is true when menu labels and pulldown may be shown (blocked by modal dialogs
// and by the fullscreen file preview, which has no pulldown menus).
func (m Model) MenuBarInteractive() bool {
	return !m.HideMenuBar && !m.ModalDialogOpen() && m.ViewMode != ViewFilePreview
}

// Render paints model into the screen's logical cell buffer. The caller must invoke
// screen.Show() (or screen.Sync()) to flush to the terminal.
func Render(screen tcell.Screen, model Model, styles theme.Theme) {
	width, height := screen.Size()
	layout := CalculateLayout(width, height, model.MenuBarLayoutReserved(), PanelWidthSplit{
		Zoom:              PanelZoomSplitsColumns(model.ViewMode, model.PanelZoomEnabled),
		ActivePanel:       model.ActivePanel,
		ActivePercent:     model.PanelZoomActivePercent,
		InactivePercent:   model.PanelZoomInactivePercent,
		HideInactivePanel: LayoutHideInactivePanel(model.ViewMode, model.HideInactivePanel),
	})
	primitive.Fill(screen, primitive.Rect{Width: width, Height: height}, ' ', tcell.StyleDefault)

	if layout.TooSmall {
		primitive.Text(screen, 0, 0, width, "Terminal too small", styles.MessageInfo)
		return
	}

	menus := menu.ActiveDefinitions(model.MenuDefinitions)
	showMenuBarSpinner := model.MenuBarActivitySpinner
	if model.MenuBarLayoutReserved() {
		if model.ModalDialogOpen() || model.ViewMode == ViewFilePreview {
			drawMenuBarBlank(screen, layout.Menu, styles, model.MenuBarJobs, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		} else {
			drawMenuBar(screen, layout.Menu, model.Menu, menus, styles, model.MenuBarJobs, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		}
	}
	msg := strings.TrimSpace(model.Message)
	chromeBlocked := model.PanelsChromeBlocked()
	switch model.ViewMode {
	case ViewFilePreview:
		union := MergeTwinPanelRects(layout.Left, layout.Right)
		drawFilePreviewPanel(screen, union, model.FullscreenFilePreviewDraw, styles, chromeBlocked, true, false, "", "")
	case ViewJobs:
		now := time.Now()
		drawJobsView(screen, layout, model.JobsView, model.JobsList, model.JobActivity, styles, now, chromeBlocked, model.UserHomeDir, model.JobsThroughputChartEnabled)
	case ViewCommands:
		cmdEntries := model.CommandsList
		if len(model.CommandsDisplay) > 0 {
			cmdEntries = model.CommandsDisplay
		}
		drawCommandsView(screen, layout, model.CommandsView, cmdEntries, styles, chromeBlocked, model.UserHomeDir)
	case ViewMessages:
		drawMessagesView(screen, layout, model.MessagesView, model.MessageLog, styles, chromeBlocked)
	default:
		// Theme picker: show the real left panel (normal chrome, always active) so preview matches in-browser use.
		previewTheme := model.ThemeDialog.Open
		leftChromeBlocked := chromeBlocked && !previewTheme
		leftFileListFocus := previewTheme || (model.ActivePanel == LeftPanel && model.ActiveSubFocus == SubFocusFileList)
		rightFileListFocus := model.ActivePanel == RightPanel && model.ActiveSubFocus == SubFocusFileList

		leftStripCount := model.Left.SelectionsStripCount()
		rightStripCount := model.Right.SelectionsStripCount()
		leftStripN := SelectionsStripLayoutItemCountFromCount(leftStripCount, LeftPanel, model.ActivePanel, previewTheme)
		rightStripN := SelectionsStripLayoutItemCountFromCount(rightStripCount, RightPanel, model.ActivePanel, previewTheme)
		leftFile := FileListFrameWithStripCount(layout.Left, leftStripN, model.SelectionsPanelMaxRows)
		rightFile := FileListFrameWithStripCount(layout.Right, rightStripN, model.SelectionsPanelMaxRows)
		_, leftStrip := SplitPanelColumn(layout.Left, leftStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)
		_, rightStrip := SplitPanelColumn(layout.Right, rightStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)

		leftSelectionsBottomHint := leftStripCount > 0 && leftStripN == 0
		rightSelectionsBottomHint := rightStripCount > 0 && rightStripN == 0
		leftStripVisible := leftStrip.Height > 0
		rightStripVisible := rightStrip.Height > 0
		leftSelectionSizeOnFileBottom := model.Left.SelectedPathCount() > 0 && !leftStripVisible
		rightSelectionSizeOnFileBottom := model.Right.SelectedPathCount() > 0 && !rightStripVisible
		leftSelectionSizeOnStripBottom := model.Left.SelectedPathCount() > 0 && leftStripVisible
		rightSelectionSizeOnStripBottom := model.Right.SelectedPathCount() > 0 && rightStripVisible

		inactiveID := RightPanel
		if model.ActivePanel == RightPanel {
			inactiveID = LeftPanel
		}
		showLeftPreview := !model.HideInactivePanel && model.FilePreviewDraw.Open && inactiveID == LeftPanel
		showRightPreview := !model.HideInactivePanel && model.FilePreviewDraw.Open && inactiveID == RightPanel

		leftOtherPanelPath := model.Right.PathString()
		rightOtherPanelPath := model.Left.PathString()

		syncDriver := model.SyncDriverPanelID()
		quickViewDriver := model.QuickViewDriverPanelID()
		if layout.Left.Width > 0 && showLeftPreview {
			pvFocused := model.ActiveSubFocus == SubFocusInactivePreview
			drawFilePreviewPanel(screen, leftFile, model.FilePreviewDraw, styles, leftChromeBlocked, pvFocused,
				model.QuickViewDisplayActive(), model.Left.PathString(), model.UserHomeDir)
		} else if layout.Left.Width > 0 {
			drawPanel(screen, leftFile, model.PanelForFileListRender(LeftPanel), leftFileListFocus, leftChromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.showPanelDiskUsage(LeftPanel), LeftPanel, model.JobPathMarks, syncDriver, quickViewDriver, model.MetaResults[LeftPanel], model.ShrunkenShowsNameOnly, leftSelectionsBottomHint, model.HideInactivePanel, model.ActivePanel, leftOtherPanelPath, leftSelectionSizeOnFileBottom)
		}
		if layout.Left.Width > 0 && leftStrip.Height > 0 {
			leftStripFocused := model.ActivePanel == LeftPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, leftStrip, model.Left, leftStripFocused, leftChromeBlocked, styles, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, leftSelectionSizeOnStripBottom)
		}
		if layout.Right.Width > 0 && showRightPreview {
			pvFocused := model.ActiveSubFocus == SubFocusInactivePreview
			drawFilePreviewPanel(screen, rightFile, model.FilePreviewDraw, styles, chromeBlocked, pvFocused,
				model.QuickViewDisplayActive(), model.Right.PathString(), model.UserHomeDir)
		} else if layout.Right.Width > 0 {
			drawPanel(screen, rightFile, model.PanelForFileListRender(RightPanel), rightFileListFocus, chromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.showPanelDiskUsage(RightPanel), RightPanel, model.JobPathMarks, syncDriver, quickViewDriver, model.MetaResults[RightPanel], model.ShrunkenShowsNameOnly, rightSelectionsBottomHint, model.HideInactivePanel, model.ActivePanel, rightOtherPanelPath, rightSelectionSizeOnFileBottom)
		}
		if layout.Right.Width > 0 && rightStrip.Height > 0 {
			rightStripFocused := model.ActivePanel == RightPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, rightStrip, model.Right, rightStripFocused, chromeBlocked, styles, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, rightSelectionSizeOnStripBottom)
		}
	}
	if model.Menu.Open && model.MenuBarInteractive() {
		drawPulldownMenu(screen, layout, model.Menu, menus, styles)
	}
	switch model.PrimaryModal() {
	case PrimaryModalTheme:
		dialog.DrawThemeDialog(screen, layout, model.ThemeDialog, styles)
	case PrimaryModalTransfer:
		dialog.DrawTransferDialog(screen, layout, model.TransferDialog, styles)
	case PrimaryModalFlatten:
		dialog.DrawFlattenDialog(screen, layout, model.FlattenDialog, styles)
	case PrimaryModalConflict:
		dialog.DrawConflictDialog(screen, layout, model.ConflictDialog, styles, model.UserHomeDir)
	case PrimaryModalQuit:
		dialog.DrawQuitConfirmDialog(screen, layout, model.QuitConfirm, styles)
	}
	if model.ConfigDialog.Open {
		dialog.DrawConfigDialog(screen, layout, model.ConfigDialog, styles)
	}
	if model.SortDialog.Open {
		dialog.DrawSortDialog(screen, layout, model.SortDialog, styles)
	}
	if model.UserMenu.Open {
		dialog.DrawUserMenuDialog(screen, layout, model.UserMenu, styles)
	}
	if model.ListingFormatDialog.Open {
		dialog.DrawListingFormatDialog(screen, layout, model.ListingFormatDialog, styles)
	}
	if model.PathPicker.Open {
		dialog.DrawPathPickerDialog(screen, layout, model.PathPicker, styles)
	}
	if model.HistoryDialog.Open {
		dialog.DrawHistoryDialog(screen, layout, model.HistoryDialog, styles)
	}
	if model.SFTPConnectDialog.Open {
		dialog.DrawSFTPConnectDialog(screen, layout, model.SFTPConnectDialog, styles)
	}
	if model.FindDialog.Open {
		selectionLabel := FindDialogSelectionSizePadded(
			model.FindDialog,
			model.DiskUsage,
			model.DiskUsageDescendIntoMountPoints,
			model.DiskUsageGoduIgnore,
			styles.SymbolWorking(),
		)
		dialog.DrawFindDialog(screen, layout, model.FindDialog, styles, model.ShowFileIcons, FindListIconLeadingWidth(model.ShowFileIcons), PaintFindDialogRowIcon, selectionLabel)
	}
	if model.GroupSelect.Open {
		dialog.DrawGroupSelectDialog(screen, layout, model.GroupSelect, styles)
	}
	if model.MetaDialog.Open {
		dialog.DrawMetaDialog(screen, layout, model.MetaDialog, styles)
	}
	if model.FileDialog.Open {
		dialog.DrawFileDialog(screen, layout, model.FileDialog, styles, model.ShowFileIcons, DeleteListIconLeadingWidth(model.ShowFileIcons), PaintDeleteDialogRowIcon)
	}
	if model.HostKeyDialog.Open {
		dialog.DrawHostKeyDialog(screen, layout, model.HostKeyDialog, styles)
	}
	if model.HelpView.Open {
		dialog.DrawHelpDialog(screen, layout, model.HelpView, styles)
	}
	drawFooter(screen, layout.Footer, styles, model.FooterKeys)
	// Transient status must be drawn after modal chrome so it is not overwritten (e.g. theme picker).
	// Draw before the generic message dialog so that modal stays the topmost curated surface when both apply.
	if msg != "" && layout.Footer.Height > 0 {
		row := Rect{X: 0, Y: layout.Footer.Y - 1, Width: layout.Width, Height: 1}
		drawStatusMessageOverlay(screen, row, msg, model.MessageUrgency, styles)
	}
	if model.StashRestoreDialog.Open {
		dialog.DrawStashRestoreDialog(screen, layout, model.StashRestoreDialog, styles)
	}
	if model.MessageDialog.Open {
		dialog.DrawMessageDialog(screen, layout, model.MessageDialog, styles)
	}
	// Caller must invoke screen.Show() or screen.Sync() so the terminal updates.
}

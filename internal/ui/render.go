package ui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/panelcarousel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog"
	"github.com/paranoidi/paras-commander/internal/ui/geom"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

const (
	PrimaryPanel = iota
	SecondaryPanel
)

// SubFocus areas within the active browser column (file list vs selections strip),
// or keyboard focus on the inactive-column file preview when it is open.
const (
	SubFocusFileList = iota
	SubFocusSelectionsStrip
	// SubFocusInactivePreview is only used while FilePreview is open (inactive column shows preview).
	SubFocusInactivePreview
)

// MetaColumnState is one active meta column on a panel.
type MetaColumnState struct {
	EntryName   string
	ColumnTitle string
	Order       int
	Results     map[string]string // abs path → raw stdout
}

// Model is the renderable subset of application state.
type Model struct {
	Primary        panel.State
	Secondary      panel.State
	ActivePanel    int
	ActiveSubFocus int // SubFocus*; applies to ActivePanel when ViewBrowser.
	// SplitOrientation selects side-by-side (SplitHorizontal) or stacked (SplitVertical) twin panes.
	SplitOrientation SplitOrientation
	// HideInactivePanel gives the active column full width and hides the inactive twin panel.
	HideInactivePanel bool
	// SyncFollowEnabled gates latched panel sync. When true, SyncFollowPanel
	// (PrimaryPanel or SecondaryPanel) names the driver whose caret moves auto-load
	// the highlighted directory into the inactive panel.
	SyncFollowEnabled bool
	SyncFollowPanel   int
	// DestinationTargetPrimary / DestinationTargetSecondary mark a panel as the resolved
	// Copy/Move/Flatten destination (green border), updated on the debounced destination
	// path validation tick (see App.updateDestinationTargetPanels).
	DestinationTargetPrimary   bool
	DestinationTargetSecondary bool
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
	CommandsDisplay     []CommandRunEntry
	MessagesView        MessagesViewState
	MessageLog          []MessageLogEntry
	CompareView         CompareViewState
	CompareSnapshot     comparepkg.Snapshot
	CompareMergeDialog  dialog.CompareMergeDialogState
	CompareFilterDialog dialog.CompareFilterDialogState
	DedupView           DedupViewState
	DedupSnapshot       comparepkg.DedupSnapshot
	DedupList           []DedupRow
	DedupCopiesList     []DedupRow
	// HideMenuBar mirrors !ui.show_menu_bar: when true, the top menu row is omitted and panels extend upward.
	HideMenuBar bool
	// ShowFileIcons mirrors ui.show_file_icons (Nerd Font glyphs before file names).
	ShowFileIcons bool
	// CarouselLayout mirrors [ui].carousel_split and carousel_show_size.
	CarouselLayout panelcarousel.Layout
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
	// PanelScrollbar mirrors [ui].panel_scrollbar (none, thumb, bar).
	PanelScrollbar uiscrollbar.Style
	// PanelScrollbarInactive mirrors [ui].panel_scrollbar_inactive.
	PanelScrollbarInactive bool
	// JobsThroughputChartEnabled mirrors [jobs].throughput_chart_enabled (strip + graph off when false).
	JobsThroughputChartEnabled bool
	// UserHomeDir is filepath.Clean(os.UserHomeDir()); empty skips ~ substitution in panel titles.
	UserHomeDir string
	// DiskUsageShown enables proportional disk-usage bars after the user starts a scan.
	DiskUsageShown bool
	// DiskUsagePanelID stores the panel (PrimaryPanel/SecondaryPanel) that initiated the current disk usage scan (pending tint only).
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
	SpinPhase               uint8
	Menu                    menu.State
	MenuDefinitions         []menu.Definition
	ThemeDialog             dialog.ThemeDialogState
	ConfigDialog            dialog.ConfigDialogState
	DebounceCalibrateDialog dialog.DebounceCalibrateDialogState
	SortDialog              dialog.SortDialogState
	ListingFormatDialog     dialog.ListingFormatDialogState
	GroupSelect             dialog.GroupSelectState
	PathPicker              dialog.PathPickerState
	HistoryDialog           dialog.HistoryDialogState
	SFTPConnectDialog       dialog.SFTPConnectDialogState
	FindDialog              dialog.FindDialogState
	MetaDialog              dialog.MetaDialogState
	UserMenu                dialog.UserMenuDialogState
	// MetaResults holds per-panel active meta columns (nil/empty = meta not active).
	MetaResults [2][]MetaColumnState
	// FilePreview is the live inactive-panel preview state (mutate only under App.commandsMu).
	FilePreview FilePreviewState
	// FilePreviewDraw is a snapshot copied in App.render before ui.Render (no locks in ui).
	FilePreviewDraw FilePreviewState
	// CarouselFilePreview is the live carousel child-column file preview (mutate only under App.commandsMu).
	CarouselFilePreview FilePreviewState
	// CarouselFilePreviewDraw is a snapshot copied in App.render before ui.Render.
	CarouselFilePreviewDraw FilePreviewState
	// QuickViewEnabled is true while Shift+F3 / menu "Quick view" is latched for QuickViewPanel.
	QuickViewEnabled bool
	// QuickViewPanel is the driver panel (PrimaryPanel or SecondaryPanel) when QuickViewEnabled, else -1.
	QuickViewPanel int
	// QuickViewDirOverlay holds a transient directory listing for quick-view directory preview (paint only).
	QuickViewDirOverlay panel.State
	// QuickViewDirOverlayActive is true when QuickViewDirOverlay should replace the inactive file list.
	QuickViewDirOverlayActive bool
	// QuickViewDirOverlayPanelID is PrimaryPanel or SecondaryPanel for the inactive column, or -1 when inactive.
	QuickViewDirOverlayPanelID int
	// QuickViewDirOverlayVisualHold retains the last dir-overlay snapshot during a folder→file debounce
	// transition. While true the inactive column continues to paint the held dir listing instead of the
	// loading file-preview chrome, preventing a blank/spinner flash before file content arrives.
	QuickViewDirOverlayVisualHold      bool
	QuickViewDirOverlayVisualHoldPanel panel.State
	// FullscreenFilePreview is the full-screen file view state (mutate only under App.commandsMu).
	FullscreenFilePreview FilePreviewState
	// FullscreenFilePreviewDraw is a snapshot for ViewFilePreview rendering.
	FullscreenFilePreviewDraw FilePreviewState
	// FullscreenFilePreviewSearchField is the "/" query editor while Search.Editing is true.
	FullscreenFilePreviewSearchField dialog.FileDialogField
	// FullscreenFilePreviewRawMarkdown is true while file.view.toggle-raw has switched the
	// fullscreen preview of a markdown file to raw Chroma-highlighted source instead of
	// rendered markdown. Reset to false whenever a new fullscreen preview opens. Only affects
	// the fullscreen target — quick view and carousel previews always render.
	FullscreenFilePreviewRawMarkdown bool
	// FilePreviewThemePicker is the inline theme list on the right side of F3 file view.
	FilePreviewThemePicker dialog.FilePreviewThemePickerState
	HelpView               dialog.HelpViewState
	FileDialog             dialog.FileDialogState
	TransferDialog         dialog.TransferDialogState
	FlattenDialog          dialog.FlattenDialogState
	ConflictDialog         dialog.ConflictDialogState
	HostKeyDialog          dialog.HostKeyDialogState
	QuitConfirm            dialog.QuitConfirmState
	AmbiguousTransfer      dialog.AmbiguousTransferState
	DedupEmptyDirsConfirm  dialog.DedupEmptyDirsConfirmState
	StashRestoreDialog     dialog.StashRestoreDialogState
	MessageDialog          dialog.MessageDialogState
	DedupProgressDialog    dialog.DedupProgressDialogState
	CommandOutputDialog    dialog.CommandOutputDialogState
	Message                string
	MessageUrgency         MessageUrgency
	FooterKeys             []menu.FunctionKey
	// MenuBarPermission is Unix mode text for the active panel cursor row (e.g. "drwxr-xr-x"); empty when none.
	MenuBarPermission string
	// MenuBarJobsAttention is the core jobs/conflict label (e.g. "! 1"); the menu bar pads it with
	// spaces on both sides for themed backgrounds and separates it from the activity spinner.
	MenuBarJobsAttention string
	// MenuBarJobs is the jobs queue + progress snapshot for the menu-bar gap (App.render).
	MenuBarJobs MenuBarJobsStrip
	// TerminalPanel is the embedded terminal panel strip state (above the footer, browser view only).
	TerminalPanel TerminalPanelState
}

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer (see package dialog).

// PrimaryModal returns the active primary modal, in the same priority order as Render.
func (m *Model) PrimaryModal() dialog.PrimaryModal {
	switch {
	case m.ThemeDialog.Open:
		return dialog.PrimaryModalTheme
	case m.ConflictDialog.Open:
		return dialog.PrimaryModalConflict
	case m.TransferDialog.Open:
		return dialog.PrimaryModalTransfer
	case m.FlattenDialog.Open:
		return dialog.PrimaryModalFlatten
	case m.QuitConfirm.Open:
		return dialog.PrimaryModalQuit
	case m.AmbiguousTransfer.Open:
		return dialog.PrimaryModalAmbiguousTransfer
	case m.DedupEmptyDirsConfirm.Open:
		return dialog.PrimaryModalDedupEmptyDirs
	default:
		return dialog.PrimaryModalNone
	}
}

// SyncDriverPanelID returns the PrimaryPanel/SecondaryPanel id that drives latched panel sync,
// or -1 when sync is disabled. The result is intended for renderers that need a sentinel
// they can compare against the panel they are about to draw.
func (m *Model) SyncDriverPanelID() int {
	if !m.SyncFollowEnabled {
		return -1
	}
	if m.SyncFollowPanel != PrimaryPanel && m.SyncFollowPanel != SecondaryPanel {
		return -1
	}
	return m.SyncFollowPanel
}

// showPanelDiskUsage is true when proportional disk-usage bars apply to panelID's listing.
func (m *Model) showPanelDiskUsage(panelID int) bool {
	if !m.DiskUsageShown || m.DiskUsage == nil {
		return false
	}
	listingPath := ""
	switch panelID {
	case PrimaryPanel:
		listingPath = m.Primary.PathString()
	case SecondaryPanel:
		listingPath = m.Secondary.PathString()
	default:
		return false
	}
	return panel.ListingPathInDiskUsageScanScope(listingPath, m.DiskUsageScanOrigin, m.DiskUsageScanRoots)
}

// quickViewDriverPanel resolves the latched quick-view driver panel.
func (m *Model) quickViewDriverPanel() int {
	if !m.QuickViewEnabled {
		return -1
	}
	if m.QuickViewPanel == PrimaryPanel || m.QuickViewPanel == SecondaryPanel {
		return m.QuickViewPanel
	}
	if m.ActivePanel == PrimaryPanel || m.ActivePanel == SecondaryPanel {
		return m.ActivePanel
	}
	return -1
}

// renderSubFocus returns ActiveSubFocus for focus styling, or -1 while the
// embedded terminal panel owns keyboard focus (panels then render inactive).
func (m *Model) renderSubFocus() int {
	if m.TerminalPanel.Visible && m.TerminalPanel.Focused {
		return -1
	}
	return m.ActiveSubFocus
}

// QuickViewDisplayActive is true when the inactive column should show quick-view preview.
func (m *Model) QuickViewDisplayActive() bool {
	if !m.QuickViewEnabled {
		return false
	}
	driver := m.quickViewDriverPanel()
	return driver >= 0 && m.ActivePanel == driver
}

// QuickViewDriverPanelID returns the PrimaryPanel/SecondaryPanel id that shows the quick-view
// bottom indicator while quick view is latched, or -1 when disabled.
func (m *Model) QuickViewDriverPanelID() int {
	return m.quickViewDriverPanel()
}

// InactiveColumnShowsFilePreview reports whether the inactive twin column should paint
// file-preview chrome instead of a file listing. During quick-view file preview the
// inactive column stays on preview even when preview state is briefly closed (e.g. between
// pause/resume), avoiding a one-frame file-panel title with volume stats that flickers
// before the filename label.
func (m *Model) InactiveColumnShowsFilePreview(inactivePanelID int) bool {
	if m.HideInactivePanel {
		return false
	}
	if m.QuickViewDisplayActive() && !m.QuickViewDirOverlayActive {
		// During a folder→file debounce transition the dir overlay snapshot is kept visible
		// until the file preview has content; show the dir panel (not file-preview chrome).
		if m.QuickViewDirOverlayVisualHold {
			return false
		}
		return true
	}
	return m.FilePreviewDraw.Open
}

// inactivePanelID returns PrimaryPanel or SecondaryPanel for the inactive column.
func (m *Model) inactivePanelID() int {
	if m.ActivePanel == SecondaryPanel {
		return PrimaryPanel
	}
	return SecondaryPanel
}

// PanelForFileListRender returns the panel state to paint in the file list. During quick-view
// directory preview the inactive column uses QuickViewDirOverlay; real Left/Right paths stay
// unchanged for cross-panel open indicators and for restore when quick view is turned off.
func (m *Model) PanelForFileListRender(panelID int) panel.State {
	if m.QuickViewDisplayActive() && m.QuickViewDirOverlayActive && panelID == m.QuickViewDirOverlayPanelID {
		return m.QuickViewDirOverlay
	}
	if m.QuickViewDisplayActive() && m.QuickViewDirOverlayVisualHold && panelID == m.inactivePanelID() {
		return m.QuickViewDirOverlayVisualHoldPanel
	}
	switch panelID {
	case PrimaryPanel:
		return m.Primary
	case SecondaryPanel:
		return m.Secondary
	default:
		return panel.State{}
	}
}

// PanelsChromeBlocked reports when file/jobs panel chrome should use panel.blocked.*
// styles because a menu or modal has taken focus.
func (m *Model) PanelsChromeBlocked() bool {
	if m.Menu.Open {
		return true
	}
	return m.ModalDialogOpen()
}

// ModalDialogOpen reports modals that block normal navigation and hide the menu bar row.
func (m *Model) ModalDialogOpen() bool {
	if m.PrimaryModal() != dialog.PrimaryModalNone {
		return true
	}
	if m.SortDialog.Open || m.ListingFormatDialog.Open || m.ConfigDialog.Open || m.DebounceCalibrateDialog.Open || m.GroupSelect.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.SFTPConnectDialog.Open || m.FindDialog.Open || m.MetaDialog.Open || m.HelpView.Open || m.FileDialog.Open || m.HostKeyDialog.Open || m.MessageDialog.Open || m.DedupProgressDialog.Open || m.StashRestoreDialog.Open || m.UserMenu.Open || m.CommandOutputDialog.Open {
		return true
	}
	return false
}

// QuickFilterStartBlocked reports UI states where the quick filter must not open from a plain printable key
// in normal input mode (same modal/menu set as the legacy shouldStartFilter guard).
func (m *Model) QuickFilterStartBlocked() bool {
	if m.Menu.Open {
		return true
	}
	return m.MessageDialog.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.SFTPConnectDialog.Open || m.FindDialog.Open ||
		m.MetaDialog.Open || m.ThemeDialog.Open || m.SortDialog.Open ||
		m.ListingFormatDialog.Open ||
		m.ConfigDialog.Open || m.DebounceCalibrateDialog.Open || m.GroupSelect.Open || m.FileDialog.Open || m.HostKeyDialog.Open ||
		m.TransferDialog.Open || m.FlattenDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.AmbiguousTransfer.Open || m.StashRestoreDialog.Open || m.UserMenu.Open ||
		m.CommandOutputDialog.Open || m.DedupProgressDialog.Open || m.DedupEmptyDirsConfirm.Open
}

// AuxiliaryViewDialogKeysBlocked reports transfer/conflict/quit dialogs plus the pulldown menu that block
// dedicated Jobs/Commands view keyboard handling. inputMode checks this only after earlier cases have ruled
// out other modals.
func (m *Model) AuxiliaryViewDialogKeysBlocked() bool {
	return m.TransferDialog.Open || m.FlattenDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.AmbiguousTransfer.Open || m.StashRestoreDialog.Open || m.DedupEmptyDirsConfirm.Open || m.Menu.Open
}

// MenuBarLayoutReserved is true when the top row is reserved for the menu strip (config show_menu_bar).
func (m *Model) MenuBarLayoutReserved() bool {
	return !m.HideMenuBar
}

// MenuBarInteractive is true when menu labels and pulldown may be shown (blocked by modal dialogs
// and the fullscreen file preview, which has no pulldown menus).
func (m *Model) MenuBarInteractive() bool {
	return !m.HideMenuBar && !m.ModalDialogOpen() && m.ViewMode != ViewFilePreview
}

// Render paints model into the screen's logical cell buffer. The caller must invoke
// screen.Show() (or screen.Sync()) to flush to the terminal.
func Render(screen tcell.Screen, model Model, styles theme.Theme) {
	width, height := screen.Size()
	// Fullscreen file preview hides the menu entirely and reclaims its row (filename sits there, borderless).
	reserveMenu := model.MenuBarLayoutReserved() && model.ViewMode != ViewFilePreview
	// Full-screen views reclaim the terminal strip (must match App.terminalLayoutRows).
	terminalRows := 0
	if model.TerminalPanel.Visible && model.ViewMode == ViewBrowser {
		terminalRows = model.TerminalPanel.Rows
	}
	layout := geom.CalculateLayoutWithOrientation(geom.LayoutInput{
		Width:       width,
		Height:      height,
		ShowMenuBar: reserveMenu,
		Split: PanelPaneSplit{
			Zoom:              PanelZoomSplitsColumns(model.ViewMode, model.PanelZoomEnabled),
			ActivePanel:       model.ActivePanel,
			ActivePercent:     model.PanelZoomActivePercent,
			InactivePercent:   model.PanelZoomInactivePercent,
			HideInactivePanel: LayoutHideInactivePanel(model.ViewMode, model.HideInactivePanel),
		},
		Orientation:  model.SplitOrientation,
		TerminalRows: terminalRows,
	})
	primitive.Fill(screen, primitive.Rect{Width: width, Height: height}, ' ', tcell.StyleDefault)

	if layout.TooSmall {
		primitive.Text(screen, 0, 0, width, "Terminal too small", styles.MessageInfo)
		return
	}

	menus := menu.ActiveDefinitions(model.MenuDefinitions)
	showMenuBarSpinner := model.MenuBarActivitySpinner
	if reserveMenu {
		if model.ModalDialogOpen() {
			drawMenuBarBlank(screen, layout.Menu, styles, model.MenuBarJobs, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		} else {
			drawMenuBar(screen, layout.Menu, model.Menu, menus, styles, model.MenuBarJobs, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		}
	}
	msg := strings.TrimSpace(model.Message)
	chromeBlocked := model.PanelsChromeBlocked()
	switch model.ViewMode {
	case ViewFilePreview:
		union := MergeTwinPanelRects(layout.Primary, layout.Secondary, model.SplitOrientation)
		previewRect, pickerRect := SplitFullscreenPreviewRects(union, model.FilePreviewThemePicker.Open, model.FilePreviewThemePicker.Choices)
		if model.FullscreenFilePreviewDraw.Search.Editing {
			if previewRect.Height > 0 {
				previewRect.Height--
			}
		}
		drawFilePreviewPanel(screen, previewRect, model.FullscreenFilePreviewDraw, styles, chromeBlocked, true, false, false, true, "", "")
		if model.FullscreenFilePreviewDraw.Search.Editing && layout.Footer.Height > 0 {
			drawFilePreviewSearchBar(screen, Rect{X: 0, Y: layout.Footer.Y - 1, Width: layout.Width, Height: 1},
				model.FullscreenFilePreviewSearchField, styles)
		}
		if model.FilePreviewThemePicker.Open && pickerRect.Width > 0 {
			dialog.DrawFilePreviewThemePicker(screen, pickerRect, model.FilePreviewThemePicker, styles)
		}
	case ViewJobs:
		now := time.Now()
		drawJobsView(screen, layout, model.JobsView, model.JobsList, model.JobActivity, styles, now, chromeBlocked, model.UserHomeDir, model.JobsThroughputChartEnabled)
	case ViewCommands:
		cmdEntries := model.CommandsList
		if len(model.CommandsDisplay) > 0 {
			cmdEntries = model.CommandsDisplay
		}
		drawCommandsView(screen, layout, model.CommandsView, cmdEntries, styles, chromeBlocked, model.UserHomeDir)
	case ViewCompare:
		filtered := comparepkg.FilteredRows(model.CompareSnapshot, model.CompareView.Filter)
		drawCompareView(screen, layout, model.CompareView,
			compareViewData{Snap: model.CompareSnapshot, Rows: filtered, Primary: model.Primary, Secondary: model.Secondary},
			styles, chromeBlocked, model.UserHomeDir, model.SplitOrientation)
		if model.CompareMergeDialog.Open {
			dialog.DrawCompareMergeDialog(screen, layout, model.CompareMergeDialog, styles, model.UserHomeDir)
		}
		if model.CompareFilterDialog.Open {
			dialog.DrawCompareFilterDialog(screen, layout, model.CompareFilterDialog, styles)
		}
	case ViewDedup:
		drawDedupView(screen, layout, model.DedupView, model.DedupSnapshot, model.DedupList, model.DedupCopiesList, styles, chromeBlocked, model.UserHomeDir, model.SplitOrientation)
	case ViewMessages:
		drawMessagesView(screen, layout, model.MessagesView, model.MessageLog, styles, chromeBlocked, model.SplitOrientation)
	default:
		// Theme picker: show the real left panel (normal chrome, always active) so preview matches in-browser use.
		previewTheme := model.ThemeDialog.Open
		primaryChromeBlocked := chromeBlocked && !previewTheme
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
		var cursorNameHintFallback CursorNameHintFallback
		if layout.Primary.Width > 0 && showLeftPreview {
			pvFocused := model.renderSubFocus() == SubFocusInactivePreview
			drawFilePreviewPanel(screen, primaryFile, model.FilePreviewDraw, styles, primaryChromeBlocked, pvFocused,
				model.QuickViewDisplayActive(), false, false, model.Primary.PathString(), model.UserHomeDir)
		} else if layout.Primary.Width > 0 {
			drawPanel(screen, primaryFile, model.PanelForFileListRender(PrimaryPanel),
				PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
				PanelContext{
					PanelID: PrimaryPanel, FileListActive: primaryFileListFocus, ChromeBlocked: primaryChromeBlocked,
					ActivePanel: model.ActivePanel, OtherPanelPath: primaryOtherPanelPath,
					HideInactivePanel: model.HideInactivePanel, SyncDriverPanelID: syncDriver, QuickViewDriverPanelID: quickViewDriver,
					SplitOrientation: model.SplitOrientation, SelectionsBottomHint: primarySelectionsBottomHint,
					ShowSelectionSizeOnBottom: primarySelectionSizeOnFileBottom,
					IsTransferTarget:          model.DestinationTargetPrimary,
					CursorNameHintFallbackOut: cursorNameHintFallbackOut(primaryFileListFocus, &cursorNameHintFallback),
				},
				PanelDisplayConfig{
					ShowIcons: model.ShowFileIcons, UserHomeDir: model.UserHomeDir,
					Painter: model.DiskUsage, DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints,
					DiskUsageGoduIgnore: model.DiskUsageGoduIgnore, ShowDiskUsage: model.showPanelDiskUsage(PrimaryPanel),
					JobMarks: model.JobPathMarks, MetaColumns: model.MetaResults[PrimaryPanel],
					ShrunkenShowsNameOnly: model.ShrunkenShowsNameOnly, ScrollbarShowInactive: model.PanelScrollbarInactive,
					CarouselLayout: model.CarouselLayout, CarouselFilePreview: model.CarouselFilePreviewDraw,
				})
		}
		if layout.Primary.Width > 0 && leftStrip.Height > 0 {
			leftStripFocused := model.ActivePanel == PrimaryPanel && model.renderSubFocus() == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, leftStrip, model.Primary, leftStripFocused, primaryChromeBlocked, SelectionsStripOpts{
				Styles: styles, UserHomeDir: model.UserHomeDir, Painter: model.DiskUsage,
				DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints, DiskUsageGoduIgnore: model.DiskUsageGoduIgnore,
				ShowSelectionSizeOnBottom: leftSelectionSizeOnStripBottom, ScrollbarStyle: model.PanelScrollbar,
				ScrollbarShowInactive: model.PanelScrollbarInactive, PanelFileListActive: primaryFileListFocus,
			})
		}
		if layout.Secondary.Width > 0 && showRightPreview {
			pvFocused := model.renderSubFocus() == SubFocusInactivePreview
			drawFilePreviewPanel(screen, secondaryFile, model.FilePreviewDraw, styles, chromeBlocked, pvFocused,
				model.QuickViewDisplayActive(), false, false, model.Secondary.PathString(), model.UserHomeDir)
		} else if layout.Secondary.Width > 0 {
			drawPanel(screen, secondaryFile, model.PanelForFileListRender(SecondaryPanel),
				PanelStyleConfig{Styles: styles, ScrollbarStyle: model.PanelScrollbar},
				PanelContext{
					PanelID: SecondaryPanel, FileListActive: secondaryFileListFocus, ChromeBlocked: chromeBlocked,
					ActivePanel: model.ActivePanel, OtherPanelPath: secondaryOtherPanelPath,
					HideInactivePanel: model.HideInactivePanel, SyncDriverPanelID: syncDriver, QuickViewDriverPanelID: quickViewDriver,
					SplitOrientation: model.SplitOrientation, SelectionsBottomHint: secondarySelectionsBottomHint,
					ShowSelectionSizeOnBottom: secondarySelectionSizeOnFileBottom,
					IsTransferTarget:          model.DestinationTargetSecondary,
					CursorNameHintFallbackOut: cursorNameHintFallbackOut(secondaryFileListFocus, &cursorNameHintFallback),
				},
				PanelDisplayConfig{
					ShowIcons: model.ShowFileIcons, UserHomeDir: model.UserHomeDir,
					Painter: model.DiskUsage, DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints,
					DiskUsageGoduIgnore: model.DiskUsageGoduIgnore, ShowDiskUsage: model.showPanelDiskUsage(SecondaryPanel),
					JobMarks: model.JobPathMarks, MetaColumns: model.MetaResults[SecondaryPanel],
					ShrunkenShowsNameOnly: model.ShrunkenShowsNameOnly, ScrollbarShowInactive: model.PanelScrollbarInactive,
					CarouselLayout: model.CarouselLayout, CarouselFilePreview: model.CarouselFilePreviewDraw,
				})
		}
		if layout.Secondary.Width > 0 && rightStrip.Height > 0 {
			rightStripFocused := model.ActivePanel == SecondaryPanel && model.renderSubFocus() == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, rightStrip, model.Secondary, rightStripFocused, chromeBlocked, SelectionsStripOpts{
				Styles: styles, UserHomeDir: model.UserHomeDir, Painter: model.DiskUsage,
				DiskUsageDescendIntoMountPoints: model.DiskUsageDescendIntoMountPoints, DiskUsageGoduIgnore: model.DiskUsageGoduIgnore,
				ShowSelectionSizeOnBottom: rightSelectionSizeOnStripBottom, ScrollbarStyle: model.PanelScrollbar,
				ScrollbarShowInactive: model.PanelScrollbarInactive, PanelFileListActive: secondaryFileListFocus,
			})
		}
		if model.TerminalPanel.Visible && layout.Terminal.Height > 0 {
			drawTerminalPanel(screen, layout.Terminal, model.TerminalPanel, styles)
		}
		drawCursorNameHintScreenFallback(screen, layout, &cursorNameHintFallback, model.TerminalPanel.Visible)
	}
	if model.Menu.Open && model.MenuBarInteractive() {
		drawPulldownMenu(screen, layout, model.Menu, menus, styles)
	}
	switch model.PrimaryModal() {
	case dialog.PrimaryModalTheme:
		dialog.DrawThemeDialog(screen, layout, model.ThemeDialog, styles)
	case dialog.PrimaryModalTransfer:
		dialog.DrawTransferDialog(screen, layout, model.TransferDialog, styles)
	case dialog.PrimaryModalFlatten:
		dialog.DrawFlattenDialog(screen, layout, model.FlattenDialog, styles)
	case dialog.PrimaryModalConflict:
		dialog.DrawConflictDialog(screen, layout, model.ConflictDialog, styles, model.UserHomeDir)
	case dialog.PrimaryModalQuit:
		dialog.DrawQuitConfirmDialog(screen, layout, model.QuitConfirm, styles)
	case dialog.PrimaryModalAmbiguousTransfer:
		dialog.DrawAmbiguousTransferDialog(screen, layout, model.AmbiguousTransfer, styles, model.UserHomeDir, model.ShowFileIcons, DialogListIconLeadingWidth(model.ShowFileIcons), PaintDeleteDialogRowIcon)
	case dialog.PrimaryModalDedupEmptyDirs:
		dialog.DrawDedupEmptyDirsConfirmDialog(screen, layout, model.DedupEmptyDirsConfirm, styles, model.ShowFileIcons, DialogListIconLeadingWidth(model.ShowFileIcons), PaintDedupEmptyDirsConfirmRowIcon)
	}
	if model.ConfigDialog.Open {
		dialog.DrawConfigDialog(screen, layout, model.ConfigDialog, styles)
	}
	if model.DebounceCalibrateDialog.Open {
		dialog.DrawDebounceCalibrateDialog(screen, layout, model.DebounceCalibrateDialog, styles)
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
			&model.FindDialog,
			model.DiskUsage,
			model.DiskUsageDescendIntoMountPoints,
			model.DiskUsageGoduIgnore,
			styles.SymbolWorking(),
		)
		dialog.DrawFindDialog(screen, layout, model.FindDialog, styles, model.ShowFileIcons, DialogListIconLeadingWidth(model.ShowFileIcons), PaintFindDialogRowIcon, selectionLabel)
	}
	if model.GroupSelect.Open {
		dialog.DrawGroupSelectDialog(screen, layout, model.GroupSelect, styles)
	}
	if model.MetaDialog.Open {
		dialog.DrawMetaDialog(screen, layout, model.MetaDialog, styles)
	}
	if model.FileDialog.Open {
		dialog.DrawFileDialog(screen, layout, model.FileDialog, styles, model.ShowFileIcons, DialogListIconLeadingWidth(model.ShowFileIcons), PaintDeleteDialogRowIcon)
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
		msgY := layout.Footer.Y - 1
		if model.TerminalPanel.Visible && layout.Terminal.Height > 0 {
			// The terminal panel occupies the row directly above the footer; paint the
			// transient message over the panel's top row instead (message wins).
			msgY = layout.Terminal.Y
		}
		row := Rect{X: 0, Y: msgY, Width: layout.Width, Height: 1}
		drawStatusMessageOverlay(screen, row, msg, model.MessageUrgency, styles)
	}
	if model.StashRestoreDialog.Open {
		dialog.DrawStashRestoreDialog(screen, layout, model.StashRestoreDialog, styles)
	}
	if model.MessageDialog.Open {
		dialog.DrawMessageDialog(screen, layout, model.MessageDialog, styles)
	}
	if model.DedupProgressDialog.Open {
		dialog.DrawDedupProgressDialog(screen, layout, model.DedupProgressDialog, model.DedupSnapshot, styles, model.UserHomeDir)
	}
	if model.CommandOutputDialog.Open {
		dialog.DrawCommandOutputDialog(screen, layout, model.CommandOutputDialog, styles)
	}
	// Caller must invoke screen.Show() or screen.Sync() so the terminal updates.
}

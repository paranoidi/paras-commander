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
	JobActivity            map[string][]string
	CommandsView           CommandsViewState
	CommandsList           []CommandRunEntry
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
	// Layout uses it only in the file browser; see PanelZoomSplitsColumns.
	PanelZoomEnabled bool
	// PanelZoomActivePercent / PanelZoomInactivePercent mirror [ui] panel_zoom_* (sum 100 when zoom enabled).
	PanelZoomActivePercent   int
	PanelZoomInactivePercent int
	// ShrunkenShowsNameOnly mirrors ui.shrunken_shows_name_only (narrow panels may hide trailing listing columns).
	ShrunkenShowsNameOnly bool
	// UserHomeDir is filepath.Clean(os.UserHomeDir()); empty skips ~ substitution in panel titles.
	UserHomeDir string
	// DiskUsageShown enables proportional disk-usage bars after the user starts a scan.
	DiskUsageShown bool
	// DiskUsagePanelID stores the panel (LeftPanel/RightPanel) that initiated the current disk usage scan.
	// Only this panel renders disk-usage bars.
	DiskUsagePanelID int
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
	MetaDialog          MetaDialogState
	UserMenu            UserMenuDialogState
	// MetaResults holds per-panel command output keyed by entry path (nil = meta not active).
	MetaResults [2]map[string]string
	// FilePreview is the live inactive-panel preview state (mutate only under App.commandsMu).
	FilePreview FilePreviewState
	// FilePreviewDraw is a snapshot copied in App.render before ui.Render (no locks in ui).
	FilePreviewDraw FilePreviewState
	// QuickViewEnabled mirrors whether Shift+F3 / menu "Quick view" keeps the inactive column in preview mode.
	QuickViewEnabled bool
	// FullscreenFilePreview is the full-screen file view state (mutate only under App.commandsMu).
	FullscreenFilePreview FilePreviewState
	// FullscreenFilePreviewDraw is a snapshot for ViewFilePreview rendering.
	FullscreenFilePreviewDraw FilePreviewState
	HelpView                  HelpViewState
	FileDialog                FileDialogState
	TransferDialog            TransferDialogState
	ConflictDialog            ConflictDialogState
	QuitConfirm               QuitConfirmState
	MessageDialog             MessageDialogState
	Message                   string
	MessageUrgency            MessageUrgency
	FooterKeys                []menu.FunctionKey
	// MenuBarPermission is Unix mode text for the active panel cursor row (e.g. "drwxr-xr-x"); empty when none.
	MenuBarPermission string
	// MenuBarJobsAttention is the core jobs/conflict label (e.g. "! 1"); the menu bar pads it with
	// spaces on both sides for themed backgrounds and separates it from the activity spinner.
	MenuBarJobsAttention string
}

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer (see package dialog).

// PrimaryModal returns the active primary modal, in the same priority order as Render.
func (m Model) PrimaryModal() PrimaryModal {
	switch {
	case m.ThemeDialog.Open:
		return PrimaryModalTheme
	case m.TransferDialog.Open:
		return PrimaryModalTransfer
	case m.ConflictDialog.Open:
		return PrimaryModalConflict
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
	if m.SortDialog.Open || m.ListingFormatDialog.Open || m.ConfigDialog.Open || m.GroupSelect.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.MetaDialog.Open || m.HelpView.Open || m.FileDialog.Open || m.MessageDialog.Open || m.UserMenu.Open {
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
	return m.MessageDialog.Open || m.PathPicker.Open || m.HistoryDialog.Open ||
		m.MetaDialog.Open || m.ThemeDialog.Open || m.SortDialog.Open ||
		m.ListingFormatDialog.Open ||
		m.ConfigDialog.Open || m.GroupSelect.Open || m.FileDialog.Open ||
		m.TransferDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.UserMenu.Open
}

// AuxiliaryViewDialogKeysBlocked reports transfer/conflict/quit dialogs plus the pulldown menu that block
// dedicated Jobs/Commands view keyboard handling. inputMode checks this only after earlier cases have ruled
// out other modals.
func (m Model) AuxiliaryViewDialogKeysBlocked() bool {
	return m.TransferDialog.Open || m.ConflictDialog.Open || m.QuitConfirm.Open || m.Menu.Open
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

// Render draws the full screen.
func Render(screen tcell.Screen, model Model, styles theme.Theme) {
	width, height := screen.Size()
	layout := CalculateLayout(width, height, model.MenuBarLayoutReserved(), PanelWidthSplit{
		Zoom:            PanelZoomSplitsColumns(model.ViewMode, model.PanelZoomEnabled),
		ActivePanel:     model.ActivePanel,
		ActivePercent:   model.PanelZoomActivePercent,
		InactivePercent: model.PanelZoomInactivePercent,
	})
	primitive.Fill(screen, primitive.Rect{Width: width, Height: height}, ' ', tcell.StyleDefault)

	if layout.TooSmall {
		primitive.Text(screen, 0, 0, width, "Terminal too small", styles.StatusInfo)
		screen.Show()
		return
	}

	menus := menu.ActiveDefinitions(model.MenuDefinitions)
	showMenuBarSpinner := model.MenuBarActivitySpinner
	permW := menuBarRightTailRuneCount(model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner)
	if model.MenuBarLayoutReserved() {
		if model.ModalDialogOpen() || model.ViewMode == ViewFilePreview {
			drawMenuBarBlank(screen, layout.Menu, styles, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		} else {
			drawMenuBar(screen, layout.Menu, model.Menu, menus, styles, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		}
	}
	msg := strings.TrimSpace(model.Message)
	chromeBlocked := model.PanelsChromeBlocked()
	switch model.ViewMode {
	case ViewFilePreview:
		union := MergeTwinPanelRects(layout.Left, layout.Right)
		drawFilePreviewPanel(screen, union, model.FullscreenFilePreviewDraw, styles, chromeBlocked, true)
	case ViewJobs:
		now := time.Now()
		drawJobsView(screen, layout, model.JobsView, model.JobsList, model.JobActivity, styles, now, chromeBlocked, model.UserHomeDir)
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

		leftStripN := SelectionsStripLayoutItemCount(&model.Left, LeftPanel, model.ActivePanel, previewTheme)
		rightStripN := SelectionsStripLayoutItemCount(&model.Right, RightPanel, model.ActivePanel, previewTheme)
		leftFile, leftStrip := SplitPanelColumn(layout.Left, leftStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)
		rightFile, rightStrip := SplitPanelColumn(layout.Right, rightStripN, model.SelectionsPanelMaxRows, MinFileListContentRows)

		leftSelectionsBottomHint := model.Left.SelectionsStripCount() > 0 && leftStripN == 0
		rightSelectionsBottomHint := model.Right.SelectionsStripCount() > 0 && rightStripN == 0

		inactiveID := RightPanel
		if model.ActivePanel == RightPanel {
			inactiveID = LeftPanel
		}
		showLeftPreview := model.FilePreviewDraw.Open && inactiveID == LeftPanel
		showRightPreview := model.FilePreviewDraw.Open && inactiveID == RightPanel

		syncDriver := model.SyncDriverPanelID()
		if showLeftPreview {
			pvFocused := model.ActiveSubFocus == SubFocusInactivePreview
			drawFilePreviewPanel(screen, leftFile, model.FilePreviewDraw, styles, leftChromeBlocked, pvFocused)
		} else {
			drawPanel(screen, leftFile, model.Left, leftFileListFocus, leftChromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.DiskUsageShown && model.DiskUsagePanelID == LeftPanel, LeftPanel, model.JobsList, syncDriver, model.MetaResults[LeftPanel], model.ShrunkenShowsNameOnly, leftSelectionsBottomHint)
		}
		if leftStrip.Height > 0 {
			leftStripFocused := model.ActivePanel == LeftPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, leftStrip, model.Left, leftStripFocused, leftChromeBlocked, styles, model.UserHomeDir)
		}
		if showRightPreview {
			pvFocused := model.ActiveSubFocus == SubFocusInactivePreview
			drawFilePreviewPanel(screen, rightFile, model.FilePreviewDraw, styles, chromeBlocked, pvFocused)
		} else {
			drawPanel(screen, rightFile, model.Right, rightFileListFocus, chromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.DiskUsageShown && model.DiskUsagePanelID == RightPanel, RightPanel, model.JobsList, syncDriver, model.MetaResults[RightPanel], model.ShrunkenShowsNameOnly, rightSelectionsBottomHint)
		}
		if rightStrip.Height > 0 {
			rightStripFocused := model.ActivePanel == RightPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, rightStrip, model.Right, rightStripFocused, chromeBlocked, styles, model.UserHomeDir)
		}
	}
	if model.Menu.Open && model.MenuBarInteractive() {
		drawPulldownMenu(screen, layout, model.Menu, menus, styles)
	}
	drawFooter(screen, layout.Footer, styles, model.FooterKeys)
	switch model.PrimaryModal() {
	case PrimaryModalTheme:
		dialog.DrawThemeDialog(screen, layout, model.ThemeDialog, styles)
	case PrimaryModalTransfer:
		dialog.DrawTransferDialog(screen, layout, model.TransferDialog, styles)
	case PrimaryModalConflict:
		dialog.DrawConflictDialog(screen, layout, model.ConflictDialog, styles)
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
	if model.GroupSelect.Open {
		dialog.DrawGroupSelectDialog(screen, layout, model.GroupSelect, styles)
	}
	if model.PathPicker.Open {
		dialog.DrawPathPickerDialog(screen, layout, model.PathPicker, styles)
	}
	if model.HistoryDialog.Open {
		dialog.DrawHistoryDialog(screen, layout, model.HistoryDialog, styles)
	}
	if model.MetaDialog.Open {
		dialog.DrawMetaDialog(screen, layout, model.MetaDialog, styles)
	}
	if model.FileDialog.Open {
		dialog.DrawFileDialog(screen, layout, model.FileDialog, styles)
	}
	if model.HelpView.Open {
		dialog.DrawHelpDialog(screen, layout, model.HelpView, styles)
	}
	// Transient status must be drawn after modal chrome so it is not overwritten (e.g. theme picker).
	// Draw before the generic message dialog so that modal stays the topmost curated surface when both apply.
	if msg != "" {
		if model.MenuBarLayoutReserved() && layout.Menu.Width > 0 {
			reserveEnd := layout.Menu.X
			if !model.ModalDialogOpen() && model.ViewMode != ViewFilePreview {
				reserveEnd = menuBarMenusEndX(layout.Menu, menus, permW)
			}
			// Use full menu row width so the banner reaches the right edge; it paints over permission text.
			drawStatusMessageOverlay(screen, layout.Menu, reserveEnd, 1, 0, msg, model.MessageUrgency, styles)
		} else if !model.MenuBarLayoutReserved() || layout.Menu.Width == 0 {
			drawStatusMessageOverlay(screen, layout.Footer, layout.Footer.X, 0, 0, msg, model.MessageUrgency, styles)
		}
	}
	if model.MessageDialog.Open {
		dialog.DrawMessageDialog(screen, layout, model.MessageDialog, styles)
	}
	screen.Show()
}

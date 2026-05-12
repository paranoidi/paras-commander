package ui

import (
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/primitive"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/menu"
)

const (
	LeftPanel = iota
	RightPanel
)

// SubFocus areas within the active browser column (file list vs selections strip).
const (
	SubFocusFileList = iota
	SubFocusSelectionsStrip
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
	// HideMenuBar mirrors !ui.show_menu_bar: when true, the top menu row is omitted and panels extend upward.
	HideMenuBar bool
	// ShowFileIcons mirrors ui.show_file_icons (Nerd Font glyphs before file names).
	ShowFileIcons bool
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
	SpinPhase       uint8
	Menu            menu.State
	MenuDefinitions []menu.Definition
	ThemeDialog     ThemeDialogState
	ConfigDialog    ConfigDialogState
	SortDialog      SortDialogState
	GroupSelect     GroupSelectState
	PathPicker      PathPickerState
	HistoryDialog   HistoryDialogState
	MetaDialog      MetaDialogState
	// MetaResults holds per-panel command output keyed by entry path (nil = meta not active).
	MetaResults    [2]map[string]string
	HelpView       HelpViewState
	FileDialog     FileDialogState
	TransferDialog TransferDialogState
	ConflictDialog ConflictDialogState
	QuitConfirm    QuitConfirmState
	MessageDialog  MessageDialogState
	Message        string
	MessageUrgency MessageUrgency
	FooterKeys     []menu.FunctionKey
	// MenuBarPermission is Unix mode text for the active panel cursor row (e.g. "drwxr-xr-x"); empty when none.
	MenuBarPermission string
	// MenuBarJobsAttention is the core jobs/conflict label (e.g. "! 1"); the menu bar pads it with
	// spaces on both sides for themed backgrounds and separates it from the activity spinner.
	MenuBarJobsAttention string
}

// ThemeChoice describes a theme option rendered in the selection dialog.
type ThemeChoice struct {
	Name  string
	Label string
}

// MessageDialogState is a generic modal with a title, body text, and OK or OK/Cancel buttons.
type MessageDialogState struct {
	Open        bool
	Title       string
	Message     string
	TwoButtons  bool
	ButtonFocus int // 0=OK, 1=Cancel when TwoButtons
}

// ThemeDialogState is the renderable state for the theme selection modal.
type ThemeDialogState struct {
	Open        bool
	Selected    int
	Focus       int // 0=list, 1=OK button, 2=Cancel button
	CurrentName string
	Choices     []ThemeChoice
}

// ConfigDialogState is the Options → Configuration modal (runtime UI toggles persisted to config.toml).
type ConfigDialogState struct {
	Open          bool
	ShowFileIcons bool
	Focus         int // 0=checkbox, 1=OK, 2=Cancel
}

// SortDialogState is the renderable state for the sort configuration modal.
type SortDialogState struct {
	Open                  bool
	SortMode              panel.SortMode
	SortReverse           bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool
	Focus                 int // 0-3=radios, 4=disk idle sort, 5=reverse, 6=dirs first, 7=OK, 8=Cancel
	PanelID               int // LeftPanel or RightPanel
}

// GroupSelectState is the renderable state for the group selection input modal.
type GroupSelectState struct {
	Open             bool
	Text             string
	Mode             string // "select" or "unselect"
	FilesOnly        bool
	CaseSensitive    bool
	UseShellPatterns bool
	Focus            int // 0=pattern input, 1=Files only, 2=Case sensitive, 3=Using shell patterns, 4=OK, 5=Cancel
}

// PathPickerPurpose selects what happens when the user confirms a path in the picker.
type PathPickerPurpose int

const (
	// PathPickerPurposeNavigate jumps the active panel to the selected directory (bookmarks menu).
	PathPickerPurposeNavigate PathPickerPurpose = iota
	// PathPickerPurposeApplyTransferDestination writes the path into the copy/move destination field.
	PathPickerPurposeApplyTransferDestination
	// PathPickerPurposeApplyFileDialogField writes the path into FileDialog.Fields[FileFieldIndex].Value.
	PathPickerPurposeApplyFileDialogField
)

// PathPickerItem is one fuzzy-listed row (display Line + filesystem Path).
type PathPickerItem struct {
	Line string
	Path string
}

// PathPickerState is a fuzzy-filtered list dialog (bookmarks, quick path, etc.).
type PathPickerState struct {
	Open           bool
	Title          string
	Purpose        PathPickerPurpose
	FileFieldIndex int // when Purpose == PathPickerPurposeApplyFileDialogField
	Query          string
	QueryCursor    int // rune offset of caret within Query (0..len(runes))
	QueryScroll    int // first visible rune offset for horizontal scrolling
	Items          []PathPickerItem
	Ranked         []int // indices into Items (rank order)
	MatchRanges    [][]search.Range
	Selected       int // index into Ranked
	ListScroll     int // first visible row index into Ranked
	Focus          int // 0=list+query, 1=OK, 2=Cancel
	// QueryPathInvalid is true after a debounced check when the filter looks like a path and os.Lstat fails.
	QueryPathInvalid bool
	// QueryPathCheckPending is true until debounced validation runs after Query changed.
	QueryPathCheckPending bool
}

// HistoryDialogState is a fuzzy picker over one panel’s navigation history paths.
type HistoryDialogState struct {
	Open         bool
	PanelID      int      // LeftPanel or RightPanel
	Paths        []string // snapshot when dialog opened
	CurrentIndex int      // snapshot HistoryIndex when dialog opened
	DisplayLines []string // per-row UI text ("* path" / "  path"); len == len(Paths)
	Query        string
	Ranked       []int            // indices into Paths / DisplayLines
	MatchRanges  [][]search.Range // len == len(Paths); highlights on DisplayLines
	Selected     int              // index into Ranked
	ListScroll   int
	Focus        int // 0=list+query, 1=OK, 2=Cancel
}

// MetaEntry is one selectable command in the meta picker dialog.
type MetaEntry struct {
	Name        string
	Description string
}

// MetaDialogState is the radio-button picker for selecting a meta command to run on panel entries.
// Entries always has "None" as first item (index 0). Focus 0..len(Entries)-1 are radio rows;
// len(Entries) is OK; len(Entries)+1 is Cancel.
type MetaDialogState struct {
	Open     bool
	PanelID  int
	Entries  []MetaEntry // first entry is always {Name:"none", Description:"None (clear)"}
	Selected int         // index into Entries (0 = None)
	Focus    int         // 0..len(Entries)-1 radio items, len = OK, len+1 = Cancel
}

// HelpEntry is one row in the full-screen help view.
type HelpEntry struct {
	ActionID string // keymap action id (e.g. file.copy)
	Title    string // "Copy"
	Keys     string // "F5"
	Section  string // "File operations"
	Context  string // optional context, e.g. "Browser"
	Search   string // concatenated text for fuzzy matching
}

// HelpViewState holds state for the centered help dialog with fuzzy search.
type HelpViewState struct {
	Open        bool
	Query       string
	Entries     []HelpEntry
	Ranked      []int            // indices into Entries (rank order)
	MatchRanges [][]search.Range // len == len(Entries); highlight ranges on Search
	Selected    int              // index into Ranked
	ListScroll  int              // first visible row index into Ranked
	Focus       int              // 0=list+fiter, 1=Close button
}

// FileDialogType identifies which file operation dialog is active.
type FileDialogType int

const (
	FileDialogNone FileDialogType = iota
	FileDialogRename
	FileDialogMkdir
	FileDialogDelete
	FileDialogChmod
	FileDialogChown
	FileDialogSymlink
	FileDialogHardlink
	FileDialogAddBookmark
	FileDialogRunForEach
)

// FileDialogField is a single input field in a file operation dialog.
type FileDialogField struct {
	Label   string
	Value   string
	Prefill string
	Cursor  int
	// PrefillPending is true while Value still shows the suggested default (Prefill).
	// The first printable character clears and replaces; Backspace/arrow/home/end/delete
	// commits the suggestion so the user edits it in place.
	PrefillPending bool
	// PathPicker enables a trailing glyph and path-picker sub-focus on the input row.
	PathPicker bool
	// PickerFocused is true when the trailing path-picker glyph has sub-focus (file dialogs).
	PickerFocused bool
}

// MkdirAction identifies the post-mkdir action chosen via radio buttons in the mkdir dialog.
// Only meaningful when DialogType == FileDialogMkdir and MkdirShowActions == true.
type MkdirAction int

const (
	MkdirActionCreate           MkdirAction = iota // just create the directory
	MkdirActionCreateCopySelect                    // create and queue copy of current selection into it
	MkdirActionCreateMoveSelect                    // create and queue move of current selection into it
)

// FileDialogState holds state for any file operation dialog.
type FileDialogState struct {
	Open         bool
	DialogType   FileDialogType
	Fields       []FileDialogField
	FocusedField int
	Message      string
	// RunForEachPaths / RunForEachDir apply when DialogType == FileDialogRunForEach (targets resolved at dialog open).
	RunForEachPaths []string
	RunForEachDir   string
	// MkdirShowActions enables the extra "Create / Create and copy selected / Create and move selected" radio
	// rows below the directory-name input. Set by openMkdirDialog when the active panel has selections.
	MkdirShowActions bool
	// MkdirAction is the currently selected mkdir post-action (only meaningful when MkdirShowActions is true).
	MkdirAction MkdirAction
}

// PrimaryModal identifies which exclusive modal occupies the primary dialog layer.
// Overlay modals (Sort, GroupSelect, FileDialog) may draw on top; see Render.
type PrimaryModal int

const (
	PrimaryModalNone PrimaryModal = iota
	PrimaryModalTheme
	PrimaryModalTransfer
	PrimaryModalConflict
	PrimaryModalQuit
)

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
	if m.SortDialog.Open || m.ConfigDialog.Open || m.GroupSelect.Open || m.PathPicker.Open || m.HistoryDialog.Open || m.MetaDialog.Open || m.HelpView.Open || m.FileDialog.Open || m.MessageDialog.Open {
		return true
	}
	return false
}

// MenuBarLayoutReserved is true when the top row is reserved for the menu strip (config show_menu_bar).
func (m Model) MenuBarLayoutReserved() bool {
	return !m.HideMenuBar
}

// MenuBarInteractive is true when menu labels and pulldown may be shown (blocked by modal dialogs).
func (m Model) MenuBarInteractive() bool {
	return !m.HideMenuBar && !m.ModalDialogOpen()
}

// Render draws the full screen.
func Render(screen tcell.Screen, model Model, styles theme.Theme) {
	width, height := screen.Size()
	layout := CalculateLayout(width, height, model.MenuBarLayoutReserved())
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
		if model.ModalDialogOpen() {
			drawMenuBarBlank(screen, layout.Menu, styles, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		} else {
			drawMenuBar(screen, layout.Menu, model.Menu, menus, styles, model.MenuBarJobsAttention, model.MenuBarPermission, showMenuBarSpinner, model.SpinPhase)
		}
	}
	msg := strings.TrimSpace(model.Message)
	chromeBlocked := model.PanelsChromeBlocked()
	switch model.ViewMode {
	case ViewJobs:
		now := time.Now()
		drawJobsView(screen, layout, model.JobsView, model.JobsList, model.JobActivity, styles, now, chromeBlocked, model.UserHomeDir)
	case ViewCommands:
		cmdEntries := model.CommandsList
		if len(model.CommandsDisplay) > 0 {
			cmdEntries = model.CommandsDisplay
		}
		drawCommandsView(screen, layout, model.CommandsView, cmdEntries, styles, chromeBlocked, model.UserHomeDir)
	default:
		// Theme picker: show the real left panel (normal chrome, always active) so preview matches in-browser use.
		previewTheme := model.ThemeDialog.Open
		leftChromeBlocked := chromeBlocked && !previewTheme
		leftFileListFocus := previewTheme || (model.ActivePanel == LeftPanel && model.ActiveSubFocus == SubFocusFileList)
		rightFileListFocus := model.ActivePanel == RightPanel && model.ActiveSubFocus == SubFocusFileList

		leftFile, leftStrip := SplitPanelColumn(layout.Left, model.Left.SelectionsStripCount(), model.SelectionsPanelMaxRows, minFileListContentRows)
		rightFile, rightStrip := SplitPanelColumn(layout.Right, model.Right.SelectionsStripCount(), model.SelectionsPanelMaxRows, minFileListContentRows)

		syncDriver := model.SyncDriverPanelID()
		drawPanel(screen, leftFile, model.Left, leftFileListFocus, leftChromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.DiskUsageShown && model.DiskUsagePanelID == LeftPanel, LeftPanel, model.JobsList, syncDriver, model.MetaResults[LeftPanel])
		if leftStrip.Height > 0 {
			leftStripFocused := model.ActivePanel == LeftPanel && model.ActiveSubFocus == SubFocusSelectionsStrip
			drawSelectionsStrip(screen, leftStrip, model.Left, leftStripFocused, leftChromeBlocked, styles, model.UserHomeDir)
		}
		drawPanel(screen, rightFile, model.Right, rightFileListFocus, chromeBlocked, styles, model.ShowFileIcons, model.UserHomeDir, model.DiskUsage, model.DiskUsageDescendIntoMountPoints, model.DiskUsageGoduIgnore, model.DiskUsageShown && model.DiskUsagePanelID == RightPanel, RightPanel, model.JobsList, syncDriver, model.MetaResults[RightPanel])
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
		drawThemeDialog(screen, layout, model.ThemeDialog, styles)
	case PrimaryModalTransfer:
		drawTransferDialog(screen, layout, model.TransferDialog, styles)
	case PrimaryModalConflict:
		drawConflictDialog(screen, layout, model.ConflictDialog, styles)
	case PrimaryModalQuit:
		drawQuitConfirmDialog(screen, layout, model.QuitConfirm, styles)
	}
	if model.ConfigDialog.Open {
		drawConfigDialog(screen, layout, model.ConfigDialog, styles)
	}
	if model.SortDialog.Open {
		drawSortDialog(screen, layout, model.SortDialog, styles)
	}
	if model.GroupSelect.Open {
		drawGroupSelectDialog(screen, layout, model.GroupSelect, styles)
	}
	if model.PathPicker.Open {
		drawPathPickerDialog(screen, layout, model.PathPicker, styles)
	}
	if model.HistoryDialog.Open {
		drawHistoryDialog(screen, layout, model.HistoryDialog, styles)
	}
	if model.MetaDialog.Open {
		drawMetaDialog(screen, layout, model.MetaDialog, styles)
	}
	if model.FileDialog.Open {
		drawFileDialog(screen, layout, model.FileDialog, styles)
	}
	if model.HelpView.Open {
		drawHelpDialog(screen, layout, model.HelpView, styles)
	}
	// Transient status must be drawn after modal chrome so it is not overwritten (e.g. theme picker).
	// Draw before the generic message dialog so that modal stays the topmost curated surface when both apply.
	if msg != "" {
		if model.MenuBarLayoutReserved() && layout.Menu.Width > 0 {
			reserveEnd := layout.Menu.X
			if !model.ModalDialogOpen() {
				reserveEnd = menuBarMenusEndX(layout.Menu, menus, permW)
			}
			// Use full menu row width so the banner reaches the right edge; it paints over permission text.
			drawStatusMessageOverlay(screen, layout.Menu, reserveEnd, 1, 0, msg, model.MessageUrgency, styles)
		} else if !model.MenuBarLayoutReserved() || layout.Menu.Width == 0 {
			drawStatusMessageOverlay(screen, layout.Footer, layout.Footer.X, 0, 0, msg, model.MessageUrgency, styles)
		}
	}
	if model.MessageDialog.Open {
		drawMessageDialog(screen, layout, model.MessageDialog, styles)
	}
	screen.Show()
}

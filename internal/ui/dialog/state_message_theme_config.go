package dialog

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
	"github.com/paranoidi/paras-commander/internal/uiscrollbar"
)

// ThemeChoice describes a theme option rendered in the selection dialog.
type ThemeChoice struct {
	Name  string
	Label string
}

// FilePreviewThemePickerState is the inline theme list on the right side of F3 file view.
type FilePreviewThemePickerState struct {
	Open         bool
	Choices      []ThemeChoice
	DisplayLines []string
	Query        string
	QueryCursor  int
	QueryScroll  int
	Ranked       []int
	MatchRanges  [][]search.Range
	Selected     int
	ListScroll   int
}

// MessageDialogState is a generic modal with a title, body text and a single OK button.
type MessageDialogState struct {
	Open    bool
	Title   string
	Message string
}

// ThemeDialogState is the renderable state for the theme selection modal.
type ThemeDialogState struct {
	Open        bool
	Focus       dlg.ID // ThemeFilter, dlg.OK or dlg.Cancel ("" = the filter)
	CurrentName string
	Choices     []ThemeChoice
	// Query is the fuzzy filter; Ranked holds the Choices indices that match it, best first, and
	// MatchRanges the highlight ranges per Choices index.
	Query       string
	QueryCursor int
	QueryScroll int
	Ranked      []int
	MatchRanges [][]search.Range
	Selected    int // index into Ranked
	ListScroll  int
}

// ConfigDialogState is the Options → Configuration modal (runtime UI toggles persisted to config.toml).
type ConfigDialogState struct {
	Open                   bool
	UseNerdfontIcons       bool
	ZoomActivePanel        bool
	PaneSplitStacked       bool
	ScrollMode             panel.ScrollMode
	PanelScrollbar         uiscrollbar.Style
	PanelScrollbarInactive bool
	ListFormat             panel.ListFormat
	Mouse                  bool               // [ui].mouse
	Split                  [3]FileDialogField // [carousel].split tokens: parent / center / child
	// Focus is the focused widget of ConfigDialogView ("" = the first checkbox): ConfigNerdfont,
	// ConfigZoom, ConfigHorizontalSplit, ConfigMouse, a ConfigScrollModeID / ConfigScrollbarID / ConfigListingID
	// radio, a ConfigSplitID input, dlg.OK or dlg.Cancel.
	Focus dlg.ID

	// EditStubConfirm shows the "config.toml does not exist, generate default and open it?"
	// confirmation, entered via F9 when no config.toml exists yet.
	EditStubConfirm      bool
	EditStubConfirmFocus dlg.ID // ConfigConfirmYes (the default) or ConfigConfirmNo

	// ResetDefaultsConfirm shows the "delete config.toml and reset to defaults?" confirmation,
	// entered via F8 when config.toml exists.
	ResetDefaultsConfirm      bool
	ResetDefaultsConfirmFocus dlg.ID // ConfigConfirmYes (the default) or ConfigConfirmNo
}

// PreviewSettingsDialogState is the M-F3 preview settings modal: three tri-state terminal
// capability radio groups (persisted as "auto"/"yes"/"no" in [preview]), a radio forcing the
// active graphics protocol ([preview].image_protocol), an image-metadata detail-level radio
// ([preview].image_metadata), and a video-metadata checkbox ([preview].video_metadata).
type PreviewSettingsDialogState struct {
	Open bool
	// Sixel / Kitty / KittyPlaceholder are config.PreviewTerminalCapabilityAuto/Yes/No for
	// [preview].terminal_sixel / terminal_kitty / terminal_kitty_placeholder.
	Sixel            string
	Kitty            string
	KittyPlaceholder string
	// Protocol is one of config.PreviewImageProtocolAuto/Sixel/Kitty.
	Protocol string
	// ImageMetadata is one of config.PreviewImageMetadataOff/Basic/Essentials/Full.
	ImageMetadata string
	// VideoMetadata reflects the Video metadata checkbox ([preview].video_metadata).
	VideoMetadata bool
	// Focus is the focused widget ("" = the first capability radio): a PreviewCapabilityID,
	// PreviewProtocolID or PreviewMetadataID radio, PreviewVideoMetadata, dlg.OK or dlg.Cancel.
	Focus dlg.ID
}

// SortDialogMetaRadio is one active meta column offered as a sort-target radio in the sort
// dialog. Title is the display title shown on the radio row; Name is the matching EntryName
// (SortState.MetaColumn identifies the sort target by Name, not by Title).
type SortDialogMetaRadio struct {
	Title string
	Name  string
}

// SortDialogState is the renderable state for the sort configuration modal.
type SortDialogState struct {
	Open                  bool
	SortMode              panel.SortMode
	SortReverse           bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool
	// MetaRadios holds the active meta columns, capped at len(panel.SortDialogRadios()) (one row
	// per built-in radio) so the dialog height never grows.
	MetaRadios []SortDialogMetaRadio
	// MetaColumn is the EntryName of the meta column selected in the dialog (SortMode == SortMeta).
	MetaColumn string
	// Focus is the focused widget of SortDialogView; empty focuses the first radio.
	Focus   dlg.ID
	PanelID int // PrimaryPanel or SecondaryPanel
}

// MetaCount returns the number of meta radios to show: len(MetaRadios) capped at
// len(panel.SortDialogRadios()).
func (s SortDialogState) MetaCount() int {
	n := len(s.MetaRadios)
	if cap := len(panel.SortDialogRadios()); n > cap {
		n = cap
	}
	return n
}

// ListingFormatDialogState is the renderable state for the panel listing format modal.
type ListingFormatDialogState struct {
	Open       bool
	ListFormat panel.ListFormat
	Focus      dlg.ID
	PanelID    int // PrimaryPanel or SecondaryPanel
}

// GroupSelectState is the renderable state for the group selection input modal.
type GroupSelectState struct {
	Open               bool
	Text               string
	TextCursor         int    // rune offset of caret within Text (0..len(runes))
	TextScroll         int    // first visible rune offset for horizontal scrolling
	Mode               string // "select" or "unselect"
	Context            string // "" or "panel" (default), "find", "output"
	PatternMode        panel.GroupPatternMode
	PatternCompileHint string
	FilesOnly          bool
	DirsOnly           bool
	CaseSensitive      bool
	// FullPath matches the full path instead of the basename; see GroupSelectShowsFullPath.
	FullPath           bool
	MetaColumnCount    int  // number of visible meta columns; 0 hides the meta checkboxes
	IncludeMetaColumns bool // when true and MetaColumnCount > 0, meta column values are also matched
	OnlyMetaColumns    bool // when true, match only meta column values (skip filename matching)
	// Types are the checked file-type presets (indexed FilterTypeVideos..): OR-joined and AND-ed
	// with the pattern; matched against the entry basename only.
	Types [FilterTypeCount]bool
	// Focus is the focused widget: PatternInput, a radio, checkbox, dlg.OK or dlg.Cancel (see the
	// Pattern* IDs in pattern_dialog_view.go).
	Focus dlg.ID

	// Live result preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}

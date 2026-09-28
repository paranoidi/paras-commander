package dialog

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/search"
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
	Open                   bool
	UseNerdfontIcons       bool
	ZoomActivePanel        bool
	ShrunkenShowsNameOnly  bool
	PaneSplitStacked       bool
	ScrollMode             panel.ScrollMode
	PanelScrollbar         uiscrollbar.Style
	PanelScrollbarInactive bool
	ListFormat             panel.ListFormat
	Focus                  int // 0=nerdfont icons, 1=zoom, 2=shrunken, 3=horizontal split, 4-9=scroll mode (left) / scrollbar (right), 10-12=listing format, 13=OK, 14=Cancel

	// EditStubConfirm shows the "config.toml does not exist, generate default and open it?"
	// confirmation, entered via F9 when no config.toml exists yet.
	EditStubConfirm      bool
	EditStubConfirmFocus int // 0=Yes (default), 1=No

	// ResetDefaultsConfirm shows the "delete config.toml and reset to defaults?" confirmation,
	// entered via F8 when config.toml exists.
	ResetDefaultsConfirm      bool
	ResetDefaultsConfirmFocus int // 0=Yes (default), 1=No
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
	Focus         int // 0-8=capability radios, 9-11=protocol, 12-15=metadata, 16=video, 17=OK, 18=Cancel
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
	// Focus: 0..len(panel.SortDialogRadios())-1=built-in radios, then meta radios, then disk idle
	// sort, reverse, dirs first checkboxes, OK, Cancel. See CheckboxFocus/OKFocus/CancelFocus.
	Focus   int
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

// CheckboxFocus returns the focus index of the first checkbox (disk usage idle sort), the row
// immediately after the built-in and meta radios.
func (s SortDialogState) CheckboxFocus() int { return len(panel.SortDialogRadios()) + s.MetaCount() }

// OKFocus returns the focus index of the OK button.
func (s SortDialogState) OKFocus() int { return s.CheckboxFocus() + 3 }

// CancelFocus returns the focus index of the Cancel button.
func (s SortDialogState) CancelFocus() int { return s.OKFocus() + 1 }

// ListingFormatDialogState is the renderable state for the panel listing format modal.
type ListingFormatDialogState struct {
	Open       bool
	ListFormat panel.ListFormat
	Focus      int // 0-2=radios, 3=OK, 4=Cancel
	PanelID    int // PrimaryPanel or SecondaryPanel
}

// GroupSelectState is the renderable state for the group selection input modal.
type GroupSelectState struct {
	Open               bool
	Text               string
	TextCursor         int    // rune offset of caret within Text (0..len(runes))
	TextScroll         int    // first visible rune offset for horizontal scrolling
	Mode               string // "select" or "unselect"
	Context            string // "" or "panel" (default), "find"
	PatternMode        panel.GroupPatternMode
	PatternCompileHint string
	FilesOnly          bool
	DirsOnly           bool
	CaseSensitive      bool
	// FullPath matches the full path instead of the basename. Find-dialog context only
	// (Context == "find"): panel entries all share one directory, so it would be meaningless
	// there.
	FullPath           bool
	MetaColumnCount    int  // number of visible meta columns; 0 hides the meta checkboxes
	IncludeMetaColumns bool // when true and MetaColumnCount > 0, meta column values are also matched
	OnlyMetaColumns    bool // when true, match only meta column values (skip filename matching)
	Focus              int  // 0-2=mode radios, 3=pattern, 4=files only, 5=dirs only, 6=case sensitive, 7=full path (find only), 8=include meta, 9=only meta (last two when MetaColumnCount>0), then OK, Cancel

	// Live result preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}

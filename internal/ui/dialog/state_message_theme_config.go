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
	ShowFileIcons          bool
	ZoomActivePanel        bool
	ShrunkenShowsNameOnly  bool
	PaneSplitStacked       bool
	ScrollMode             panel.ScrollMode
	PanelScrollbar         uiscrollbar.Style
	PanelScrollbarInactive bool
	ListFormat             panel.ListFormat
	Focus                  int // 0=file icons, 1=zoom, 2=shrunken, 3=horizontal split, 4-9=scroll mode (left) / scrollbar (right), 10-12=listing format, 13=OK, 14=Cancel
}

// SortDialogState is the renderable state for the sort configuration modal.
type SortDialogState struct {
	Open                  bool
	SortMode              panel.SortMode
	SortReverse           bool
	DirectoriesFirst      bool
	DiskUsageIdleSizeSort bool
	Focus                 int // 0-3=radios, 4=disk idle sort, 5=reverse, 6=dirs first, 7=OK, 8=Cancel
	PanelID               int // PrimaryPanel or SecondaryPanel
}

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
	MetaColumnCount    int  // number of visible meta columns; 0 hides the meta checkboxes
	IncludeMetaColumns bool // when true and MetaColumnCount > 0, meta column values are also matched
	OnlyMetaColumns    bool // when true, match only meta column values (skip filename matching)
	Focus              int  // 0-2=mode radios, 3=pattern, 4=files only, 5=dirs only, 6=case sensitive, 7=include meta, 8=only meta (last two when MetaColumnCount>0), then OK, Cancel

	// Live result preview shown right-aligned on the Pattern row, recomputed on every state
	// change. PreviewShow is false while the pattern is empty or fails to compile.
	PreviewFiles   int
	PreviewFolders int
	PreviewShow    bool
}

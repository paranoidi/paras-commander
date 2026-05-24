package dialog

import "github.com/paranoidi/paras-commander/internal/panel"

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
	Open                  bool
	ShowFileIcons         bool
	ZoomActivePanel       bool
	ShrunkenShowsNameOnly bool
	CenterScrolling       bool
	ListFormat            panel.ListFormat
	Focus                 int // 0=file icons, 1=zoom, 2=shrunken, 3=center scroll, 4-6=listing format radios, 7=OK, 8=Cancel
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

// ListingFormatDialogState is the renderable state for the panel listing format modal.
type ListingFormatDialogState struct {
	Open       bool
	ListFormat panel.ListFormat
	Focus      int // 0-2=radios, 3=OK, 4=Cancel
	PanelID    int // LeftPanel or RightPanel
}

// GroupSelectState is the renderable state for the group selection input modal.
type GroupSelectState struct {
	Open             bool
	Text             string
	TextCursor       int    // rune offset of caret within Text (0..len(runes))
	TextScroll       int    // first visible rune offset for horizontal scrolling
	Mode             string // "select" or "unselect"
	FilesOnly        bool
	CaseSensitive    bool
	UseShellPatterns bool
	Focus            int // 0=pattern input, 1=Files only, 2=Case sensitive, 3=Using shell patterns, 4=OK, 5=Cancel
}

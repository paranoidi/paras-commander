package theme

import (
	"github.com/gdamore/tcell/v2"
	"github.com/paranoidi/paras-commander/internal/localfs"
)

// PanelListingCursorOpts selects cursor-row styling for file-panel and carousel listings.
type PanelListingCursorOpts struct {
	ChromeBlocked    bool
	FileListActive   bool
	CarouselInactive bool
	Selected         bool
	// FilterUniqueMatch is true when the active quick filter has exactly one matching entry.
	// Activates panel.active.row.cursor.unique styling on the active panel cursor.
	FilterUniqueMatch bool
}

// PanelListingEntryStyle returns the base row style for a listing entry by type and chrome state.
func (t Theme) PanelListingEntryStyle(entryType localfs.EntryType, chromeBlocked bool) tcell.Style {
	if chromeBlocked {
		switch entryType {
		case localfs.EntryDirectory:
			return t.PanelBlockedRowDirectory
		case localfs.EntrySymlink:
			return t.PanelBlockedRowSymlink
		default:
			return t.PanelBlockedRowFile
		}
	}
	switch entryType {
	case localfs.EntryDirectory:
		return t.PanelRowDirectory
	case localfs.EntrySymlink:
		return t.PanelRowSymlink
	default:
		return t.PanelRowFile
	}
}

// PanelListingSelectedStyle returns the row style when an entry is selected but not the cursor.
func (t Theme) PanelListingSelectedStyle(chromeBlocked bool) tcell.Style {
	if chromeBlocked {
		return t.PanelBlockedRowSelected
	}
	return t.PanelRowSelected
}

// PanelListingCursorStyle returns cursor-row styling for file panels and carousel columns.
// base is the row's non-cursor style; when the theme's cursor style leaves fg unset
// (e.g. panel.inactive.row.cursor = { bg = "black" }), the base foreground and attributes
// are kept so entry-type colors (directory blue bold, symlink cyan) survive under the cursor.
func (t Theme) PanelListingCursorStyle(base tcell.Style, opts PanelListingCursorOpts) tcell.Style {
	cursor := t.panelListingCursorThemeStyle(opts)
	if fg, _, _ := cursor.Decompose(); fg == tcell.ColorDefault {
		_, bg, _ := cursor.Decompose()
		return base.Background(bg)
	}
	return cursor
}

func (t Theme) panelListingCursorThemeStyle(opts PanelListingCursorOpts) tcell.Style {
	if opts.ChromeBlocked {
		if opts.Selected {
			return t.PanelBlockedCursorSelected
		}
		return t.PanelBlockedCursor
	}
	if opts.CarouselInactive {
		if opts.Selected {
			return t.PanelCarouselInactiveCursorSelected
		}
		return t.PanelCarouselInactiveCursor
	}
	if opts.FileListActive {
		if opts.FilterUniqueMatch {
			return t.PanelCursorActiveUnique
		}
		if opts.Selected {
			return t.PanelActiveCursorSelected
		}
		return t.PanelCursorActive
	}
	if opts.Selected {
		return t.PanelInactiveCursorSelected
	}
	return t.PanelCursorInactive
}

// PanelListingCursorIconKey returns the theme key for panel.*.row.cursor icon overrides on the cursor row.
func (t Theme) PanelListingCursorIconKey(opts PanelListingCursorOpts) string {
	if opts.ChromeBlocked {
		if opts.Selected {
			return "panel.blocked.row.cursor.selected"
		}
		return "panel.blocked.row.cursor"
	}
	if opts.CarouselInactive {
		if opts.Selected {
			return "panel.carousel.inactive.row.cursor.selected"
		}
		return "panel.carousel.inactive.row.cursor"
	}
	if opts.FileListActive {
		if opts.FilterUniqueMatch {
			return "panel.active.row.cursor.unique"
		}
		if opts.Selected {
			return "panel.active.row.cursor.selected"
		}
		return "panel.active.row.cursor"
	}
	if opts.Selected {
		return "panel.inactive.row.cursor.selected"
	}
	return "panel.inactive.row.cursor"
}

package dialog

import (
	"github.com/gdamore/tcell/v2"

	"github.com/paranoidi/paras-commander/internal/theme"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Find relocate button IDs; Esc leaves the parked search parked.
const (
	FindRelocateRestore = dlg.ID("restore")
	FindRelocateNew     = dlg.ID("new")
)

// FindRelocateState asks, when Find (or Find in index) is opened somewhere other than where
// a parked one searches, whether to restore the parked search or start a new one here.
type FindRelocateState struct {
	Open    bool
	Focus   dlg.ID // FindRelocateRestore (the default) or FindRelocateNew
	Index   bool   // the parked search is Find in index
	PanelID int    // where a new search starts
	Jump    bool
	Root    string // where the parked search searches
}

// FindRelocateView is the relocate question's widget spec.
func FindRelocateView(s FindRelocateState) dlg.Spec {
	title := "Find"
	if s.Index {
		title = "Find in index"
	}
	return dlg.Spec{
		Title: title, Width: 56, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "A search is parked in"},
			{Kind: dlg.KindLabel, Label: s.Root, Fit: dlg.FitPath},
		}}}},
		Buttons: []dlg.Button{
			{ID: FindRelocateRestore, Label: "Restore", Mnemonic: 'R'},
			{ID: FindRelocateNew, Label: "New search here", Mnemonic: 'N'},
		},
	}
}

// DrawFindRelocateDialog paints the relocate question from its widget spec.
func DrawFindRelocateDialog(screen tcell.Screen, layout Layout, state FindRelocateState, styles theme.Theme) {
	dlg.Draw(screen, layout.Width, layout.Height, FindRelocateView(state), state.Focus, styles)
}

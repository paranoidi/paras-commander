package dialog

import (
	"github.com/paranoidi/paras-commander/internal/panel"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

const listingFormatPrefix = "format:"

// ListingFormatView is the listing format dialog's widget spec: one radio per format above OK /
// Cancel. Plain o / c press the buttons.
func ListingFormatView(s ListingFormatDialogState) dlg.Spec {
	lf := panel.EffectiveListFormat(s.ListFormat)
	var items []dlg.Item
	for _, r := range panel.ListFormatDialogRadios() {
		items = append(items, dlg.Item{
			ID: dlg.ID(listingFormatPrefix + r.Label), Kind: dlg.KindRadio, Label: r.Label, Mnemonic: r.Shortcut,
			On: lf == r.Format,
		})
	}
	return dlg.Spec{
		Title: "Listing format", Width: 44, MinWidth: 28, MinHeight: 7, PlainButtonKeys: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			{ID: dlg.OK, Label: "OK", Mnemonic: 'O'},
			{ID: dlg.Cancel, Label: "Cancel", Mnemonic: 'C'},
		},
	}
}

// Apply folds a radio activation from dlg.HandleKey into the state. OK and Cancel are the
// caller's to handle.
func (s *ListingFormatDialogState) Apply(ev dlg.Event) {
	for _, r := range panel.ListFormatDialogRadios() {
		if ev.ID == dlg.ID(listingFormatPrefix+r.Label) {
			s.ListFormat = r.Format
			return
		}
	}
}

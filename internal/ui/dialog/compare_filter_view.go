package dialog

import (
	comparepkg "github.com/paranoidi/paras-commander/internal/compare"
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

func compareFilterItemID(f comparepkg.Filter) dlg.ID { return dlg.IndexedID("kind:", int(f)) }

// CompareFilterView is the compare category / select-by-kind picker's widget spec: one checkbox
// per kind (Radios depends on the mode) above OK / Cancel.
func CompareFilterView(s CompareFilterDialogState) dlg.Spec {
	radios := s.Radios()
	items := make([]dlg.Item, len(radios))
	for i, r := range radios {
		items[i] = dlg.Item{
			ID: compareFilterItemID(r.Filter), Kind: dlg.KindCheckbox, Label: r.Label, Mnemonic: r.Shortcut,
			On: comparepkg.FilterChecked(s.Filter, r.Filter),
		}
	}
	return dlg.Spec{
		Title: s.Title(), Width: 32, MinWidth: 24, MinHeight: len(radios) + 4, // border, rows, separator, buttons, border
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds a checkbox activation from dlg.HandleKey into the pending filter (the "All" row is
// the master toggle, see comparepkg.ToggleFilter). OK and Cancel are the caller's to handle.
func (s *CompareFilterDialogState) Apply(ev dlg.Event) {
	for _, r := range s.Radios() {
		if ev.ID == compareFilterItemID(r.Filter) {
			s.Filter = comparepkg.ToggleFilter(s.Filter, r.Filter)
			return
		}
	}
}

package dialog

import (
	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// MetaEntryID is the widget ID of the checkbox row for entry i.
func MetaEntryID(i int) dlg.ID { return dlg.IndexedID("entry:", i) }

const metaDialogWidth = 56

// MetaDialogView is the meta column picker's widget spec: one checkbox per meta entry above OK /
// Cancel, or the wrapped Notice when there are no entries. Plain o / c press the buttons.
func MetaDialogView(s MetaDialogState) dlg.Spec {
	shortcuts := MetaEntryShortcuts(s.Entries)
	items := make([]dlg.Item, len(s.Entries))
	for i, e := range s.Entries {
		items[i] = dlg.Item{
			ID: MetaEntryID(i), Kind: dlg.KindCheckbox, Label: metaEntryDisplayLabel(e), Mnemonic: shortcuts[i],
			On: i < len(s.Checked) && s.Checked[i],
		}
	}
	if len(s.Entries) == 0 && s.Notice != "" {
		for _, line := range wrapMessageBody(s.Notice, metaDialogWidth-4) {
			items = append(items, dlg.Item{Kind: dlg.KindLabel, Label: line, Style: dlg.TextWarn})
		}
	}
	return dlg.Spec{
		Title: "Meta", Width: metaDialogWidth, MinWidth: 28, MinHeight: 5, PlainButtonKeys: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{items}}},
		Buttons: []dlg.Button{
			dlg.OKButton,
			dlg.CancelButton,
		},
	}
}

// Apply folds a checkbox activation from dlg.HandleKey into the state. OK and Cancel are the
// caller's to handle.
func (s *MetaDialogState) Apply(ev dlg.Event) {
	for i := range s.Entries {
		if ev.ID == MetaEntryID(i) && i < len(s.Checked) {
			s.Checked[i] = !s.Checked[i]
			return
		}
	}
}

package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// Stash restore button IDs. The Cancel button is dlg.Cancel.
const (
	StashReplace   = dlg.ID("replace")
	StashMerge     = dlg.ID("merge")
	StashDropStash = dlg.ID("drop-stash")
	StashDropAll   = dlg.ID("drop-all")
)

// StashRestoreView is the stash restore dialog's widget spec: two message lines above five
// buttons. The dialog widens to fit the button strip.
func StashRestoreView(StashRestoreDialogState) dlg.Spec {
	return dlg.Spec{
		Title: "Stash restore", Width: 72, MinWidth: 68, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{{
			{Kind: dlg.KindLabel, Label: "Panel has live selections and a non-empty stash."},
			{Kind: dlg.KindLabel, Label: "Choose how to resolve:"},
		}}}},
		Buttons: []dlg.Button{
			{ID: StashReplace, Label: "Replace", Mnemonic: 'R'},
			{ID: StashMerge, Label: "Merge", Mnemonic: 'M'},
			{ID: StashDropStash, Label: "Drop stash", Mnemonic: 'D'},
			{ID: StashDropAll, Label: "Drop all", Mnemonic: 'A'},
			dlg.CancelButton,
		},
	}
}

package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// DialogScreen is the screen of a dialog that is on top: its spec and the focus field that screen
// is driven with. A dialog whose sub-screens (pickers, the conflict rules, a Yes/No confirm) take
// the place of its main screen decides which one is on top in one place, its Screen method, which
// the renderer, the key handler and App.activeDialog all read.
type DialogScreen struct {
	Spec  dlg.Spec
	Focus *dlg.ID
}

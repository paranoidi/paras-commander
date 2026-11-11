package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// ArchivePasswordDialogState asks for an encrypted archive's password during an extract.
type ArchivePasswordDialogState struct {
	Open bool
	// Path is the archive.
	Path string
	// BadPassword shows that the previous password was rejected.
	BadPassword bool
	// Password is the masked input row.
	Password FileDialogField
	// Focus is the focused widget (ArchivePasswordInput, dlg.OK or dlg.Cancel); empty is the
	// first focusable one.
	Focus dlg.ID
}

// FocusOnInput reports whether the password field has focus.
func (s ArchivePasswordDialogState) FocusOnInput() bool { return s.Focus == ArchivePasswordInput }

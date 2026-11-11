package dialog

import "github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"

// ElevateDialogState asks whether a permission-denied file operation should be retried as root.
type ElevateDialogState struct {
	Open bool
	Op   string // "rename", "delete", ...
	Path string
	// NoPassword hides the password field: sudo needs none.
	NoPassword bool
	// BadPassword shows that the previous password was rejected.
	BadPassword bool
	// Password is the masked input row.
	Password FileDialogField
	// Focus is the focused widget (ElevatePassword, ElevateRetry, ElevateSkip or dlg.Cancel); empty
	// is the first focusable one.
	Focus dlg.ID
}

// HasInput reports whether the password field is shown.
func (s ElevateDialogState) HasInput() bool { return !s.NoPassword }

// FocusOnInput reports whether the password field has focus.
func (s ElevateDialogState) FocusOnInput() bool { return s.HasInput() && s.Focus == ElevatePassword }

package dialog

// ElevateButtons is the number of buttons in the elevate dialog: Elevate, Skip, Cancel.
const ElevateButtons = 3

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
	// Focus: with a password field 0 is the field and 1..3 are the buttons; without it 0..2 are the buttons.
	Focus int
}

// HasInput reports whether the password field is shown.
func (s ElevateDialogState) HasInput() bool { return !s.NoPassword }

// ButtonBase is the Focus value of the first button (Elevate).
func (s ElevateDialogState) ButtonBase() int {
	if s.HasInput() {
		return 1
	}
	return 0
}

// FocusOnInput reports whether the password field has focus.
func (s ElevateDialogState) FocusOnInput() bool { return s.HasInput() && s.Focus == 0 }

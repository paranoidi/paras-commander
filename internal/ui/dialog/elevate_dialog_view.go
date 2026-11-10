package dialog

import (
	"strings"
	"unicode/utf8"

	"github.com/paranoidi/paras-commander/internal/ui/dialog/dlg"
)

// Elevate prompt widget IDs. The Cancel button is dlg.Cancel.
const (
	ElevatePassword = dlg.ID("password")
	ElevateRetry    = dlg.ID("elevate")
	ElevateSkip     = dlg.ID("skip")
)

// ElevateView is the "retry as root" prompt's widget spec: the failed operation and path, then
// either the masked password input or the sudo question, above Elevate / Skip / Cancel. It is
// as wide as the screen allows up to 64 columns, and never below 44 where the screen has room.
func ElevateView(s ElevateDialogState) dlg.Spec {
	if s.ArchivePassword {
		return archivePasswordView(s)
	}
	head := []dlg.Item{
		{Kind: dlg.KindLabel, Label: elevateHeadline(s.Op)},
		{Kind: dlg.KindLabel, Label: s.Path, Fit: dlg.FitMiddle},
	}
	if s.BadPassword {
		head = append(head, dlg.Item{Kind: dlg.KindLabel, Label: "Incorrect password.", Style: dlg.TextError})
	}
	head = append(head, dlg.Item{Kind: dlg.KindSeparator})
	body := []dlg.Item{{Kind: dlg.KindLabel, Label: "Retry as root with sudo?"}}
	if s.HasInput() {
		body = passwordInput(s)
	}
	return dlg.Spec{
		Title: "Permission denied", Width: 64, MinWidth: 44, Unclamped: true,
		Groups: []dlg.Group{{Columns: [][]dlg.Item{head}}, {Columns: [][]dlg.Item{body}}},
		Buttons: []dlg.Button{
			{ID: ElevateRetry, Label: "Elevate", Mnemonic: 'E'},
			{ID: ElevateSkip, Label: "Skip", Mnemonic: 'S'},
			dlg.CancelButton,
		},
	}
}

// passwordInput is the "Password:" label and the masked input row.
func passwordInput(s ElevateDialogState) []dlg.Item {
	return []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Password:"},
		{
			ID: ElevatePassword, Kind: dlg.KindInput,
			Value: strings.Repeat("*", utf8.RuneCountInString(s.Password.Value)), Cursor: s.Password.Cursor, Scroll: s.Password.Scroll,
		},
	}
}

// archivePasswordView is the encrypted-archive prompt: the archive, the masked password input
// (with a "Wrong password" line after a rejected one) above OK / Cancel (Cancel skips the archive).
func archivePasswordView(s ElevateDialogState) dlg.Spec {
	head := []dlg.Item{
		{Kind: dlg.KindLabel, Label: "Encrypted archive:"},
		{Kind: dlg.KindLabel, Label: s.Path, Fit: dlg.FitMiddle},
	}
	if s.BadPassword {
		head = append(head, dlg.Item{Kind: dlg.KindLabel, Label: "Wrong password.", Style: dlg.TextError})
	}
	head = append(head, dlg.Item{Kind: dlg.KindSeparator})
	return dlg.Spec{
		Title: "Archive password", Width: 64, MinWidth: 44, Unclamped: true,
		Groups:  []dlg.Group{{Columns: [][]dlg.Item{head}}, {Columns: [][]dlg.Item{passwordInput(s)}}},
		Buttons: []dlg.Button{dlg.OKButton, dlg.CancelButton},
	}
}

func elevateHeadline(op string) string {
	if op == "" {
		return "Permission denied"
	}
	return strings.ToUpper(op[:1]) + op[1:] + " failed"
}
